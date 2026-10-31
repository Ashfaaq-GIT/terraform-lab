package provider

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"libvirt.org/go/libvirtxml"
)

// Construct complete nested Terraform objects to exercise the schema conversion.
func harithaRuleObject(t *testing.T, kind string, fields map[string]attr.Value) types.Object {
	t.Helper()
	typeMap := map[string]attr.Type{
		"src_ip": types.StringType, "src_ip_mask": types.StringType,
		"dst_ip": types.StringType, "dst_ip_mask": types.StringType,
		"src_port_start": types.Int64Type, "src_port_end": types.Int64Type,
		"dst_port_start": types.Int64Type, "dst_port_end": types.Int64Type,
	}
	if kind == "ip" {
		typeMap["protocol"] = types.StringType
	}
	if kind == "mac" {
		typeMap = map[string]attr.Type{
			"src_mac": types.StringType, "src_mac_mask": types.StringType,
			"dst_mac": types.StringType, "dst_mac_mask": types.StringType,
			"protocol_id": types.StringType,
		}
	}
	values := make(map[string]attr.Value, len(typeMap))
	for key, typ := range typeMap {
		if typ.Equal(types.Int64Type) {
			values[key] = types.Int64Null()
		} else {
			values[key] = types.StringNull()
		}
	}
	for key, value := range fields {
		values[key] = value
	}
	obj, diags := types.ObjectValue(typeMap, values)
	if diags.HasError() {
		t.Fatalf("construct %s object: %v", kind, diags)
	}
	return obj
}

func harithaEntry(t *testing.T, kind string, fields map[string]attr.Value) libvirtxml.NWFilterEntry {
	t.Helper()
	entry, err := buildNWFilterProtocolEntry(map[string]attr.Value{
		"action":    types.StringValue("accept"),
		"direction": types.StringValue("in"),
		"priority":  types.Int64Null(),
		kind:        harithaRuleObject(t, kind, fields),
	})
	if err != nil {
		t.Fatalf("build %s entry: %v", kind, err)
	}
	return entry
}

func TestNWFilterHarithaProtocolXML(t *testing.T) {
	entries := []libvirtxml.NWFilterEntry{
		harithaEntry(t, "tcp", map[string]attr.Value{
			"src_ip":         types.StringValue("192.0.2.10"),
			"dst_port_start": types.Int64Value(443),
			"dst_port_end":   types.Int64Value(445),
		}),
		harithaEntry(t, "udp", map[string]attr.Value{
			"dst_ip":         types.StringValue("198.51.100.1"),
			"dst_port_start": types.Int64Value(53),
		}),
		harithaEntry(t, "ip", map[string]attr.Value{
			"src_ip":      types.StringValue("192.0.2.0"),
			"src_ip_mask": types.StringValue("24"),
			"protocol":    types.StringValue("tcp"),
		}),
		harithaEntry(t, "mac", map[string]attr.Value{
			"src_mac": types.StringValue("52:54:00:12:34:56"),
			"dst_mac": types.StringValue("$MAC"),
		}),
	}
	source := libvirtxml.NWFilter{Name: "i2-haritha-unit", Entries: entries}
	rawXML, err := source.Marshal()
	if err != nil {
		t.Fatalf("marshal XML: %v", err)
	}
	var parsed libvirtxml.NWFilter
	if err := parsed.Unmarshal(rawXML); err != nil {
		t.Fatalf("parse XML: %v", err)
	}
	if len(parsed.Entries) != 4 {
		t.Fatalf("wanted 4 entries, got %d", len(parsed.Entries))
	}
	tcp := parsed.Entries[0].Rule.TCP
	if tcp == nil || tcp.SrcIPAddr.Str != "192.0.2.10" || tcp.DstPortStart.Uint == nil || *tcp.DstPortStart.Uint != 443 || tcp.DstPortEnd.Uint == nil || *tcp.DstPortEnd.Uint != 445 {
		t.Fatalf("incorrect TCP XML: %#v", tcp)
	}
	udp := parsed.Entries[1].Rule.UDP
	if udp == nil || udp.DstIPAddr.Str != "198.51.100.1" || udp.DstPortStart.Uint == nil || *udp.DstPortStart.Uint != 53 {
		t.Fatalf("incorrect UDP XML: %#v", udp)
	}
	ip := parsed.Entries[2].Rule.IP
	if ip == nil || ip.SrcIPAddr.Str != "192.0.2.0" || ip.SrcIPMask.Str != "24" || ip.Protocol.Str != "tcp" {
		t.Fatalf("incorrect IP XML: %#v", ip)
	}
	mac := parsed.Entries[3].Rule.MAC
	if mac == nil || mac.SrcMACAddr.Str != "52:54:00:12:34:56" || mac.DstMACAddr.Var != "MAC" {
		t.Fatalf("incorrect MAC XML: %#v", mac)
	}
}

func TestNWFilterHarithaRejectInvalidProtocols(t *testing.T) {
	tests := []struct {
		name   string
		kind   string
		fields map[string]attr.Value
		want   string
	}{
		{"negative port", "tcp", map[string]attr.Value{"dst_port_start": types.Int64Value(-1)}, "between 0 and 65535"},
		{"oversize port", "udp", map[string]attr.Value{"src_port_start": types.Int64Value(65536)}, "between 0 and 65535"},
		{"backwards range", "tcp", map[string]attr.Value{"dst_port_start": types.Int64Value(100), "dst_port_end": types.Int64Value(99)}, "end cannot be before start"},
		{"end without start", "udp", map[string]attr.Value{"src_port_end": types.Int64Value(53)}, "requires"},
		{"invalid IPv4", "ip", map[string]attr.Value{"src_ip": types.StringValue("999.1.2.3")}, "invalid src IPv4"},
		{"IPv6 in IPv4 field", "tcp", map[string]attr.Value{"dst_ip": types.StringValue("2001:db8::1")}, "invalid dst IPv4"},
		{"invalid prefix", "ip", map[string]attr.Value{"src_ip": types.StringValue("192.0.2.1"), "src_ip_mask": types.StringValue("33")}, "mask prefix"},
		{"invalid protocol", "ip", map[string]attr.Value{"protocol": types.StringValue("gre")}, "invalid IPv4 protocol"},
		{"ports without protocol", "ip", map[string]attr.Value{"src_port_start": types.Int64Value(22)}, "require protocol"},
		{"invalid MAC", "mac", map[string]attr.Value{"src_mac": types.StringValue("not-a-mac")}, "invalid Ethernet MAC"},
		{"MAC mask without address", "mac", map[string]attr.Value{"src_mac_mask": types.StringValue("ff:ff:ff:00:00:00")}, "requires"},
		{"no MAC criteria", "mac", map[string]attr.Value{}, "requires an address"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := buildNWFilterProtocolEntry(map[string]attr.Value{
				"action":    types.StringValue("accept"),
				"direction": types.StringValue("in"),
				tt.kind:     harithaRuleObject(t, tt.kind, tt.fields),
			})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("wanted %q, got %v", tt.want, err)
			}
		})
	}
}

func TestNWFilterHarithaRejectMultipleProtocols(t *testing.T) {
	_, err := buildNWFilterProtocolEntry(map[string]attr.Value{
		"action":    types.StringValue("accept"),
		"direction": types.StringValue("in"),
		"tcp":       harithaRuleObject(t, "tcp", map[string]attr.Value{}),
		"udp":       harithaRuleObject(t, "udp", map[string]attr.Value{}),
	})
	if err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("expected conflict error, got %v", err)
	}
}
