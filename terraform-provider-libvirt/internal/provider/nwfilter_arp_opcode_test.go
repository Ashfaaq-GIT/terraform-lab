package provider

import (
	"context"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"libvirt.org/go/libvirtxml"
)

func TestNWFilterImportARPOpcodeNames(t *testing.T) {
	cases := map[string]int64{
		"Request":         1,
		"Reply":           2,
		"Request_Reverse": 3,
		"Reply_Reverse":   4,
		"DRARP_Request":   5,
		"DRARP_Reply":     6,
		"DRARP_Error":     7,
		"InARP_Request":   8,
		"ARP_NAK":         9,
	}

	for name, expected := range cases {
		t.Run(name, func(t *testing.T) {
			entries := []libvirtxml.NWFilterEntry{
				{
					Rule: &libvirtxml.NWFilterRule{
						Action:    "accept",
						Direction: "in",
						ARP: &libvirtxml.NWFilterRuleARP{
							OpCode: libvirtxml.NWFilterField{Str: name},
						},
					},
				},
			}

			state, err := importNWFilterEntries(
				context.Background(), entries,
			)
			if err != nil {
				t.Fatal(err)
			}

			entry := state.Elements()[0].(types.Object)
			rule := entry.Attributes()["rule"].(types.Object)
			arp := rule.Attributes()["arp"].(types.Object)
			got := arp.Attributes()["opcode"].(types.Int64).ValueInt64()

			if got != expected {
				t.Fatalf(
					"opcode %q: expected %d, got %d",
					name, expected, got,
				)
			}
		})
	}
}

func TestNWFilterImportRejectsUnknownARPOpcode(t *testing.T) {
	entries := []libvirtxml.NWFilterEntry{
		{
			Rule: &libvirtxml.NWFilterRule{
				Action:    "accept",
				Direction: "in",
				ARP: &libvirtxml.NWFilterRuleARP{
					OpCode: libvirtxml.NWFilterField{
						Str: "UnknownOpcode",
					},
				},
			},
		},
	}

	_, err := importNWFilterEntries(context.Background(), entries)
	if err == nil || !strings.Contains(err.Error(), "opcode") {
		t.Fatalf("expected invalid ARP opcode error, got %v", err)
	}
}
