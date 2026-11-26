package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"strings"
	"testing"

	"libvirt.org/go/libvirtxml"
)

func TestNWFilterImportRejectsUnsupportedRule(t *testing.T) {
	entries := []libvirtxml.NWFilterEntry{
		{
			Rule: &libvirtxml.NWFilterRule{
				Action:    "accept",
				Direction: "in",
				VLAN:      &libvirtxml.NWFilterRuleVLAN{},
			},
		},
	}

	_, err := importNWFilterEntries(context.Background(), entries)
	if err == nil || !strings.Contains(err.Error(), "unsupported rule type") {
		t.Fatalf("expected unsupported rule error, got: %v", err)
	}
}

func TestNWFilterImportRejectsMissingICMPCode(t *testing.T) {
	icmpType := uint(8)

	entries := []libvirtxml.NWFilterEntry{
		{
			Rule: &libvirtxml.NWFilterRule{
				Action:    "accept",
				Direction: "in",
				ICMP: &libvirtxml.NWFilterRuleICMP{
					Type: libvirtxml.NWFilterField{Uint: &icmpType},
				},
			},
		},
	}

	_, err := importNWFilterEntries(context.Background(), entries)
	if err == nil || !strings.Contains(err.Error(), "requires numeric ICMP type and code") {
		t.Fatalf("expected missing ICMP code error, got: %v", err)
	}
}

// Import must accept decimal port numbers supplied directly by libvirt XML.
func TestNWFilterImportDecimalTCPPort(t *testing.T) {
	entries := []libvirtxml.NWFilterEntry{
		{
			Rule: &libvirtxml.NWFilterRule{
				Action:    "accept",
				Direction: "in",
				TCP: &libvirtxml.NWFilterRuleTCP{
					NWFilterRuleCommonPort: libvirtxml.NWFilterRuleCommonPort{
						DstPortStart: libvirtxml.NWFilterField{Str: "443"},
					},
				},
			},
		},
	}

	result, err := importNWFilterEntries(context.Background(), entries)
	if err != nil {
		t.Fatalf("decimal TCP port import failed: %v", err)
	}

	entry := result.Elements()[0].(types.Object)
	rule := entry.Attributes()["rule"].(types.Object)
	tcp := rule.Attributes()["tcp"].(types.Object)
	port := tcp.Attributes()["dst_port_start"].(types.Int64)

	if port.ValueInt64() != 443 {
		t.Fatalf("expected imported TCP port 443, got %d", port.ValueInt64())
	}
}
