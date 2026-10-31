package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"libvirt.org/go/libvirtxml"
)

// These helpers construct the same nested Terraform values used by the
// resource schema, without requiring a live libvirt daemon.
var (
	i2ParamType = types.ObjectType{AttrTypes: map[string]attr.Type{
		"name": types.StringType, "value": types.StringType,
	}}
	i2RefType = types.ObjectType{AttrTypes: map[string]attr.Type{
		"filter":     types.StringType,
		"parameters": types.ListType{ElemType: i2ParamType},
	}}
	i2ICMPType = types.ObjectType{AttrTypes: map[string]attr.Type{
		"type": types.Int64Type, "code": types.Int64Type,
	}}
	i2RuleType = types.ObjectType{AttrTypes: map[string]attr.Type{
		"action": types.StringType, "direction": types.StringType,
		"priority": types.Int64Type, "icmp": i2ICMPType,
	}}
	i2EntryType = types.ObjectType{AttrTypes: map[string]attr.Type{
		"rule": i2RuleType, "ref": i2RefType,
	}}
)

func i2Object(t *testing.T, typ types.ObjectType, values map[string]attr.Value) types.Object {
	t.Helper()
	obj, diags := types.ObjectValue(typ.AttrTypes, values)
	if diags.HasError() {
		t.Fatalf("constructing test object: %v", diags)
	}
	return obj
}

func i2List(t *testing.T, typ attr.Type, values []attr.Value) types.List {
	t.Helper()
	list, diags := types.ListValue(typ, values)
	if diags.HasError() {
		t.Fatalf("constructing test list: %v", diags)
	}
	return list
}

func i2Reference(t *testing.T, filter string, parameters ...[2]string) attr.Value {
	t.Helper()
	var params types.List
	if parameters == nil {
		params = types.ListNull(i2ParamType)
	} else {
		values := make([]attr.Value, 0, len(parameters))
		for _, pair := range parameters {
			values = append(values, i2Object(t, i2ParamType, map[string]attr.Value{
				"name": types.StringValue(pair[0]), "value": types.StringValue(pair[1]),
			}))
		}
		params = i2List(t, i2ParamType, values)
	}
	return i2Object(t, i2RefType, map[string]attr.Value{
		"filter": types.StringValue(filter), "parameters": params,
	})
}

func i2ICMPRule(t *testing.T) attr.Value {
	t.Helper()
	icmp := i2Object(t, i2ICMPType, map[string]attr.Value{
		"type": types.Int64Value(8), "code": types.Int64Value(0),
	})
	return i2Object(t, i2RuleType, map[string]attr.Value{
		"action":    types.StringValue("accept"),
		"direction": types.StringValue("in"),
		"priority":  types.Int64Null(),
		"icmp":      icmp,
	})
}

func i2Entry(t *testing.T, rule, ref attr.Value) attr.Value {
	t.Helper()
	if rule == nil {
		rule = types.ObjectNull(i2RuleType.AttrTypes)
	}
	if ref == nil {
		ref = types.ObjectNull(i2RefType.AttrTypes)
	}
	return i2Object(t, i2EntryType, map[string]attr.Value{
		"rule": rule, "ref": ref,
	})
}

func i2Model(t *testing.T, name string, entries ...attr.Value) NWFilterResourceModel {
	t.Helper()
	return NWFilterResourceModel{
		Name:    types.StringValue(name),
		Entries: i2List(t, i2EntryType, entries),
	}
}

func TestNWFilterReferencesXMLRoundTrip(t *testing.T) {
	model := i2Model(t, "i2-test-parent",
		i2Entry(t, nil, i2Reference(t, "allow-dhcp")),
		i2Entry(t, nil, i2Reference(t, "allow-dhcp-server",
			[2]string{"DHCPSERVER", "192.0.2.1"},
			[2]string{"DHCPSERVER", "192.0.2.2"})),
		i2Entry(t, i2ICMPRule(t), nil),
	)

	xml, err := nwFilterXMLFromModel(model, "")
	if err != nil {
		t.Fatalf("generate filter XML: %v", err)
	}
	var live libvirtxml.NWFilter
	if err := live.Unmarshal(xml); err != nil {
		t.Fatalf("unmarshal filter XML: %v", err)
	}
	if len(live.Entries) != 3 {
		t.Fatalf("want 3 ordered entries, got %d", len(live.Entries))
	}
	if live.Entries[0].Ref == nil || live.Entries[0].Ref.Filter != "allow-dhcp" {
		t.Fatalf("first reference incorrect: %#v", live.Entries[0])
	}
	second := live.Entries[1].Ref
	if second == nil || second.Filter != "allow-dhcp-server" || len(second.Parameters) != 2 {
		t.Fatalf("second reference/parameters incorrect: %#v", second)
	}
	if second.Parameters[0].Name != "DHCPSERVER" || second.Parameters[0].Value != "192.0.2.1" ||
		second.Parameters[1].Value != "192.0.2.2" {
		t.Fatalf("parameter values or order changed: %#v", second.Parameters)
	}
	if live.Entries[2].Rule == nil || live.Entries[2].Rule.ICMP == nil {
		t.Fatalf("third entry should be the original ICMP rule: %#v", live.Entries[2])
	}
}

func TestNWFilterReferencesRejectInvalidEntries(t *testing.T) {
	tests := []struct {
		name  string
		entry func(*testing.T) attr.Value
		want  string
	}{
		{"neither rule nor reference", func(t *testing.T) attr.Value { return i2Entry(t, nil, nil) }, "exactly one"},
		{"both rule and reference", func(t *testing.T) attr.Value { return i2Entry(t, i2ICMPRule(t), i2Reference(t, "allow-dhcp")) }, "exactly one"},
		{"blank reference", func(t *testing.T) attr.Value { return i2Entry(t, nil, i2Reference(t, " ")) }, "empty or invalid"},
		{"self reference", func(t *testing.T) attr.Value { return i2Entry(t, nil, i2Reference(t, "i2-test-parent")) }, "cannot reference its own"},
		{"blank parameter name", func(t *testing.T) attr.Value {
			return i2Entry(t, nil, i2Reference(t, "allow-dhcp", [2]string{" ", "value"}))
		}, "invalid name"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := nwFilterEntriesFromModel(i2Model(t, "i2-test-parent", tc.entry(t)))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("wanted error containing %q, got %v", tc.want, err)
			}
		})
	}
}

func TestNWFilterReferenceRefresh(t *testing.T) {
	ctx := context.Background()
	model := i2Model(t, "i2-test-parent",
		i2Entry(t, nil, i2Reference(t, "allow-dhcp-server", [2]string{"DHCPSERVER", "192.0.2.1"})),
	)
	live := []libvirtxml.NWFilterEntry{{Ref: &libvirtxml.NWFilterRef{
		Filter: "allow-dhcp-server", Parameters: []libvirtxml.NWFilterParameter{
			{Name: "DHCPSERVER", Value: "192.0.2.9"},
		},
	}}}
	refreshed, err := refreshNWFilterRefsFromXML(ctx, model.Entries, live)
	if err != nil {
		t.Fatalf("refresh references: %v", err)
	}
	model.Entries = refreshed
	entries, err := nwFilterEntriesFromModel(model)
	if err != nil {
		t.Fatalf("convert refreshed entries: %v", err)
	}
	if len(entries) != 1 || entries[0].Ref == nil || len(entries[0].Ref.Parameters) != 1 ||
		entries[0].Ref.Parameters[0].Value != "192.0.2.9" {
		t.Fatalf("updated parameter not reflected in state: %#v", entries)
	}
}
