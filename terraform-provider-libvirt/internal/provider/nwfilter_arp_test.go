package provider

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"libvirt.org/go/libvirtxml"
)

func TestNWFilterARPXMLRoundTrip(t *testing.T) {
	entry := harithaEntry(t, "arp", map[string]attr.Value{
		"src_mac":     types.StringValue("52:54:00:12:34:56"),
		"src_ip":      types.StringValue("192.0.2.10"),
		"dst_ip":      types.StringValue("192.0.2.1"),
		"dst_ip_mask": types.StringValue("24"),
		"opcode":      types.Int64Value(2),
	})

	filter := libvirtxml.NWFilter{
		Name:    "i3-haritha-arp-unit",
		Entries: []libvirtxml.NWFilterEntry{entry},
	}
	xmlText, err := filter.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(xmlText, "<arp ") {
		t.Fatalf("missing ARP XML element: %s", xmlText)
	}

	var parsed libvirtxml.NWFilter
	if err := parsed.Unmarshal(xmlText); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Entries) != 1 || parsed.Entries[0].Rule == nil {
		t.Fatalf("unexpected ARP entries: %#v", parsed.Entries)
	}
	arp := parsed.Entries[0].Rule.ARP
	if arp == nil ||
		arp.ARPSrcMACAddr.Str != "52:54:00:12:34:56" ||
		arp.ARPSrcIPAddr.Str != "192.0.2.10" ||
		arp.ARPDstIPAddr.Str != "192.0.2.1" ||
		arp.ARPDstIPMask.Str != "24" ||
		arp.OpCode.Uint == nil || *arp.OpCode.Uint != 2 {
		t.Fatalf("ARP XML round-trip mismatch: %#v", arp)
	}
}

func TestNWFilterARPRejectInvalidConfigurations(t *testing.T) {
	tests := []struct {
		name   string
		fields map[string]attr.Value
		want   string
	}{
		{"empty ARP rule", map[string]attr.Value{}, "requires an address or opcode"},
		{"invalid source MAC", map[string]attr.Value{
			"src_mac": types.StringValue("invalid-mac"),
		}, "invalid Ethernet MAC"},
		{"invalid source IPv4", map[string]attr.Value{
			"src_ip": types.StringValue("999.0.0.1"),
		}, "invalid src IPv4"},
		{"IPv6 in ARP field", map[string]attr.Value{
			"src_ip": types.StringValue("2001:db8::1"),
		}, "invalid src IPv4"},
		{"mask without address", map[string]attr.Value{
			"src_ip_mask": types.StringValue("24"),
		}, "requires src_ip"},
		{"negative opcode", map[string]attr.Value{
			"opcode": types.Int64Value(-1),
		}, "between 1 and 65535"},
		{"oversized opcode", map[string]attr.Value{
			"opcode": types.Int64Value(65536),
		}, "between 1 and 65535"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildNWFilterProtocolEntry(map[string]attr.Value{
				"action":    types.StringValue("accept"),
				"direction": types.StringValue("in"),
				"arp":       harithaRuleObject(t, "arp", tc.fields),
			})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
		})
	}
}

func TestNWFilterARPRejectMixedProtocols(t *testing.T) {
	_, err := buildNWFilterProtocolEntry(map[string]attr.Value{
		"action":    types.StringValue("accept"),
		"direction": types.StringValue("in"),
		"arp": harithaRuleObject(t, "arp", map[string]attr.Value{
			"opcode": types.Int64Value(1),
		}),
		"tcp": harithaRuleObject(t, "tcp", map[string]attr.Value{}),
	})
	if err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("expected protocol conflict, got %v", err)
	}
}
