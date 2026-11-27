package provider

import (
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"libvirt.org/go/libvirtxml"
)

func TestNWFilterIPv6XMLRoundTrip(t *testing.T) {
	entry := harithaEntry(t, "ipv6", map[string]attr.Value{
		"src_ip":      types.StringValue("2001:db8::10"),
		"src_ip_mask": types.StringValue("64"),
		"dst_ip":      types.StringValue("2001:db8::20"),
		"dst_ip_mask": types.StringValue("128"),
		"protocol":    types.StringValue("tcp"),
	})

	filter := libvirtxml.NWFilter{
		Name:    "i3-haritha-ipv6-unit",
		Entries: []libvirtxml.NWFilterEntry{entry},
	}

	xmlText, err := filter.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(xmlText, "<ipv6 ") {
		t.Fatalf("missing IPv6 XML element: %s", xmlText)
	}

	var parsed libvirtxml.NWFilter
	if err := parsed.Unmarshal(xmlText); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Entries) != 1 || parsed.Entries[0].Rule == nil {
		t.Fatalf("unexpected IPv6 entries: %#v", parsed.Entries)
	}

	ipv6 := parsed.Entries[0].Rule.IPv6
	if ipv6 == nil ||
		ipv6.SrcIPAddr.Str != "2001:db8::10" ||
		ipv6.SrcIPMask.Str != "64" ||
		ipv6.DstIPAddr.Str != "2001:db8::20" ||
		ipv6.DstIPMask.Str != "128" ||
		ipv6.Protocol.Str != "tcp" {
		t.Fatalf("IPv6 XML round-trip mismatch: %#v", ipv6)
	}
}

func TestNWFilterIPv6RejectInvalidConfigurations(t *testing.T) {
	tests := []struct {
		name   string
		fields map[string]attr.Value
		want   string
	}{
		{"empty IPv6 rule", map[string]attr.Value{},
			"requires an address or protocol"},
		{"invalid IPv6 address", map[string]attr.Value{
			"src_ip": types.StringValue("not-an-ip"),
		}, "invalid src IPv6"},
		{"IPv4 in IPv6 field", map[string]attr.Value{
			"dst_ip": types.StringValue("192.0.2.1"),
		}, "invalid dst IPv6"},
		{"mask without address", map[string]attr.Value{
			"src_ip_mask": types.StringValue("64"),
		}, "requires src_ip"},
		{"negative IPv6 prefix", map[string]attr.Value{
			"src_ip":      types.StringValue("2001:db8::1"),
			"src_ip_mask": types.StringValue("-1"),
		}, "between 0 and 128"},
		{"oversized IPv6 prefix", map[string]attr.Value{
			"dst_ip":      types.StringValue("2001:db8::2"),
			"dst_ip_mask": types.StringValue("129"),
		}, "between 0 and 128"},
		{"invalid IPv6 protocol", map[string]attr.Value{
			"protocol": types.StringValue("invalid-protocol"),
		}, "invalid IPv6 protocol"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildNWFilterProtocolEntry(map[string]attr.Value{
				"action":    types.StringValue("accept"),
				"direction": types.StringValue("in"),
				"ipv6":      harithaRuleObject(t, "ipv6", tc.fields),
			})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
		})
	}
}

func TestNWFilterIPv6RejectMixedProtocols(t *testing.T) {
	_, err := buildNWFilterProtocolEntry(map[string]attr.Value{
		"action":    types.StringValue("accept"),
		"direction": types.StringValue("in"),
		"ipv6": harithaRuleObject(t, "ipv6", map[string]attr.Value{
			"src_ip": types.StringValue("2001:db8::1"),
		}),
		"arp": harithaRuleObject(t, "arp", map[string]attr.Value{
			"opcode": types.Int64Value(1),
		}),
	})
	if err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("expected protocol conflict, got %v", err)
	}
}
