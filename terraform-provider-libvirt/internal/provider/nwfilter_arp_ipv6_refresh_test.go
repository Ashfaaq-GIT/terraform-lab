package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"libvirt.org/go/libvirtxml"
)

func TestNWFilterARPIPv6StateRefresh(t *testing.T) {
	entries := []libvirtxml.NWFilterEntry{
		harithaEntry(t, "arp", map[string]attr.Value{
			"src_mac":     types.StringValue("52:54:00:12:34:56"),
			"src_ip":      types.StringValue("192.0.2.10"),
			"dst_ip":      types.StringValue("192.0.2.1"),
			"dst_ip_mask": types.StringValue("24"),
			"opcode":      types.Int64Value(2),
		}),
		harithaEntry(t, "ipv6", map[string]attr.Value{
			"src_ip":      types.StringValue("2001:db8::10"),
			"src_ip_mask": types.StringValue("64"),
			"dst_ip":      types.StringValue("2001:db8::20"),
			"protocol":    types.StringValue("tcp"),
		}),
	}

	filter := libvirtxml.NWFilter{
		Name:    "i3-haritha-refresh-unit",
		Entries: entries,
	}
	xmlText, err := filter.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	var live libvirtxml.NWFilter
	if err := live.Unmarshal(xmlText); err != nil {
		t.Fatal(err)
	}

	state, err := importNWFilterEntries(context.Background(), live.Entries)
	if err != nil {
		t.Fatalf("refresh ARP/IPv6 rules: %v", err)
	}
	if len(state.Elements()) != 2 {
		t.Fatalf("expected 2 refreshed entries, got %d", len(state.Elements()))
	}

	getProtocol := func(index int, name string) map[string]attr.Value {
		entry := state.Elements()[index].(types.Object)
		rule := entry.Attributes()["rule"].(types.Object)
		protocol := rule.Attributes()[name].(types.Object)
		if protocol.IsNull() {
			t.Fatalf("refreshed %s rule is null", name)
		}
		return protocol.Attributes()
	}

	arp := getProtocol(0, "arp")
	for name, expected := range map[string]string{
		"src_mac":     "52:54:00:12:34:56",
		"src_ip":      "192.0.2.10",
		"dst_ip":      "192.0.2.1",
		"dst_ip_mask": "24",
	} {
		actual := arp[name].(types.String).ValueString()
		if actual != expected {
			t.Errorf("ARP %s: expected %q, got %q", name, expected, actual)
		}
	}
	if got := arp["opcode"].(types.Int64).ValueInt64(); got != 2 {
		t.Errorf("ARP opcode: expected 2, got %d", got)
	}

	ipv6 := getProtocol(1, "ipv6")
	for name, expected := range map[string]string{
		"src_ip":      "2001:db8::10",
		"src_ip_mask": "64",
		"dst_ip":      "2001:db8::20",
		"protocol":    "tcp",
	} {
		actual := ipv6[name].(types.String).ValueString()
		if actual != expected {
			t.Errorf("IPv6 %s: expected %q, got %q", name, expected, actual)
		}
	}
}

func TestNWFilterARPIPv6RefreshDetectsChangedValues(t *testing.T) {
	original := []libvirtxml.NWFilterEntry{
		harithaEntry(t, "arp", map[string]attr.Value{
			"opcode": types.Int64Value(1),
		}),
		harithaEntry(t, "ipv6", map[string]attr.Value{
			"src_ip": types.StringValue("2001:db8::1"),
		}),
	}

	arpCode := uint(2)
	original[0].Rule.ARP.OpCode = libvirtxml.NWFilterField{
		Uint: &arpCode,
	}
	original[1].Rule.IPv6.SrcIPAddr = libvirtxml.NWFilterField{
		Str: "2001:db8::2",
	}

	refreshed, err := importNWFilterEntries(context.Background(), original)
	if err != nil {
		t.Fatal(err)
	}

	arpEntry := refreshed.Elements()[0].(types.Object)
	arpRule := arpEntry.Attributes()["rule"].(types.Object)
	arp := arpRule.Attributes()["arp"].(types.Object)

	if got := arp.Attributes()["opcode"].(types.Int64).ValueInt64(); got != 2 {
		t.Errorf("expected live ARP opcode 2, got %d", got)
	}

	ipv6Entry := refreshed.Elements()[1].(types.Object)
	ipv6Rule := ipv6Entry.Attributes()["rule"].(types.Object)
	ipv6 := ipv6Rule.Attributes()["ipv6"].(types.Object)

	if got := ipv6.Attributes()["src_ip"].(types.String).ValueString(); got != "2001:db8::2" {
		t.Errorf("expected live IPv6 address 2001:db8::2, got %q", got)
	}
}
