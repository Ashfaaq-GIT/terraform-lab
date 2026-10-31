package provider

import (
	"context"
	"fmt"
	"testing"

	libvirtclient "github.com/dmacvicar/terraform-provider-libvirt/v2/internal/libvirt"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"libvirt.org/go/libvirtxml"
)

// This test uses qemu:///system or the URI provided by testAccLibvirtURI.
// It creates a dedicated filter, updates references/parameters, and destroys it.
func TestAccNWFilterResource_References(t *testing.T) {
	var originalUUID string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckNWFilterDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccNWFilterReferencesConfig("allow-dhcp", "192.0.2.1"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("libvirt_nwfilter.test", "id"),
					resource.TestCheckResourceAttr("libvirt_nwfilter.test", "entries.#", "3"),
					testAccCheckNWFilterReferences(&originalUUID, "allow-dhcp", "192.0.2.1"),
				),
			},
			{
				Config: testAccNWFilterReferencesConfig("allow-arp", "192.0.2.2"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("libvirt_nwfilter.test", "entries.0.ref.filter", "allow-arp"),
					testAccCheckNWFilterReferences(&originalUUID, "allow-arp", "192.0.2.2"),
				),
			},
		},
	})
}

func testAccCheckNWFilterReferences(originalUUID *string, firstFilter, serverIP string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources["libvirt_nwfilter.test"]
		if !ok {
			return fmt.Errorf("libvirt_nwfilter.test missing from state")
		}
		name := rs.Primary.Attributes["name"]
		stateUUID := rs.Primary.Attributes["id"]
		if name == "" || stateUUID == "" {
			return fmt.Errorf("missing filter name or UUID in state")
		}
		if *originalUUID == "" {
			*originalUUID = stateUUID
		} else if stateUUID != *originalUUID {
			return fmt.Errorf("filter UUID changed: %s -> %s", *originalUUID, stateUUID)
		}

		ctx := context.Background()
		client, err := libvirtclient.NewClient(ctx, testAccLibvirtURI())
		if err != nil {
			return fmt.Errorf("connect to libvirt: %w", err)
		}
		defer func() { _ = client.Close() }()
		filter, err := client.Libvirt().NwfilterLookupByName(name)
		if err != nil {
			return fmt.Errorf("find live filter %q: %w", name, err)
		}
		if libvirtclient.UUIDString(filter.UUID) != stateUUID {
			return fmt.Errorf("live UUID differs from Terraform UUID")
		}
		xmlDoc, err := client.Libvirt().NwfilterGetXMLDesc(filter, 0)
		if err != nil {
			return fmt.Errorf("read live XML: %w", err)
		}
		var live libvirtxml.NWFilter
		if err := live.Unmarshal(xmlDoc); err != nil {
			return fmt.Errorf("parse live XML: %w", err)
		}
		if len(live.Entries) != 3 {
			return fmt.Errorf("want three ordered entries, got %d", len(live.Entries))
		}
		if live.Entries[0].Ref == nil || live.Entries[0].Ref.Filter != firstFilter {
			return fmt.Errorf("incorrect first reference: %#v", live.Entries[0])
		}
		second := live.Entries[1].Ref
		if second == nil || second.Filter != "allow-dhcp-server" || len(second.Parameters) != 1 {
			return fmt.Errorf("incorrect second reference: %#v", second)
		}
		if second.Parameters[0].Name != "DHCPSERVER" || second.Parameters[0].Value != serverIP {
			return fmt.Errorf("incorrect server parameter: %#v", second.Parameters)
		}
		if live.Entries[2].Rule == nil || live.Entries[2].Rule.ICMP == nil {
			return fmt.Errorf("third entry must be ICMP rule")
		}
		return nil
	}
}

func testAccNWFilterReferencesConfig(firstFilter, serverIP string) string {
	return fmt.Sprintf(`
provider "libvirt" {
  uri = %[1]q
}

resource "libvirt_nwfilter" "test" {
  name = "test-i2-filter-references"

  entries = [
    {
      ref = {
        filter = %[2]q
      }
    },
    {
      ref = {
        filter = "allow-dhcp-server"
        parameters = [
          { name = "DHCPSERVER", value = %[3]q }
        ]
      }
    },
    {
      rule = {
        action    = "accept"
        direction = "in"
        icmp = { type = 8, code = 0 }
      }
    }
  ]
}
`, testAccLibvirtURI(), firstFilter, serverIP)
}
