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

func TestAccNWFilterResource_ARPIPv6(t *testing.T) {
	var originalUUID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckNWFilterDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccNWFilterARPIPv6Config("192.0.2.10", "2001:db8::10"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("libvirt_nwfilter.test", "entries.#", "2"),
					resource.TestCheckResourceAttr("libvirt_nwfilter.test", "entries.0.rule.arp.opcode", "2"),
					resource.TestCheckResourceAttr("libvirt_nwfilter.test", "entries.1.rule.ipv6.src_ip", "2001:db8::10"),
					testAccCheckARPIPv6(&originalUUID, "192.0.2.10", "2001:db8::10"),
				),
			},
			{
				Config: testAccNWFilterARPIPv6Config("192.0.2.11", "2001:db8::11"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("libvirt_nwfilter.test", "entries.0.rule.arp.src_ip", "192.0.2.11"),
					resource.TestCheckResourceAttr("libvirt_nwfilter.test", "entries.1.rule.ipv6.src_ip", "2001:db8::11"),
					testAccCheckARPIPv6(&originalUUID, "192.0.2.11", "2001:db8::11"),
				),
			},
			{
				ResourceName:      "libvirt_nwfilter.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAccNWFilterARPIPv6Config(arpIP, ipv6IP string) string {
	return fmt.Sprintf(`
provider "libvirt" { uri = %[1]q }

resource "libvirt_nwfilter" "test" {
  name = "test-nwfilter-i3-arp-ipv6"

  entries = [
    {
      rule = {
        action = "accept"
        direction = "in"
        arp = {
          src_mac = "52:54:00:12:34:56"
          src_ip = %[2]q
          opcode = 2
        }
      }
    },
    {
      rule = {
        action = "accept"
        direction = "in"
        ipv6 = {
          src_ip = %[3]q
          src_ip_mask = "64"
          protocol = "tcp"
        }
      }
    }
  ]
}
`, testAccLibvirtURI(), arpIP, ipv6IP)
}

func testAccCheckARPIPv6(
	originalUUID *string, arpIP, ipv6IP string,
) resource.TestCheckFunc {
	return func(state *terraform.State) error {
		rs := state.RootModule().Resources["libvirt_nwfilter.test"]
		if rs == nil || rs.Primary == nil {
			return fmt.Errorf("NWFilter missing from Terraform state")
		}

		uuid := rs.Primary.Attributes["id"]
		if uuid == "" {
			return fmt.Errorf("NWFilter UUID is empty")
		}
		if *originalUUID == "" {
			*originalUUID = uuid
		} else if uuid != *originalUUID {
			return fmt.Errorf("NWFilter UUID changed during update")
		}

		client, err := libvirtclient.NewClient(
			context.Background(), testAccLibvirtURI(),
		)
		if err != nil {
			return err
		}
		defer func() { _ = client.Close() }()

		filter, err := client.Libvirt().NwfilterLookupByName(
			"test-nwfilter-i3-arp-ipv6",
		)
		if err != nil {
			return err
		}

		xmlText, err := client.Libvirt().NwfilterGetXMLDesc(filter, 0)
		if err != nil {
			return err
		}

		var live libvirtxml.NWFilter
		if err := live.Unmarshal(xmlText); err != nil {
			return err
		}
		if len(live.Entries) != 2 {
			return fmt.Errorf("expected 2 live rules, got %d", len(live.Entries))
		}

		arpRule := live.Entries[0].Rule
		ipv6Rule := live.Entries[1].Rule
		if arpRule == nil || arpRule.ARP == nil {
			return fmt.Errorf("live ARP XML element missing")
		}
		if ipv6Rule == nil || ipv6Rule.IPv6 == nil {
			return fmt.Errorf("live IPv6 XML element missing")
		}
		if arpRule.ARP.ARPSrcIPAddr.Str != arpIP {
			return fmt.Errorf("live ARP address mismatch")
		}
		if ipv6Rule.IPv6.SrcIPAddr.Str != ipv6IP {
			return fmt.Errorf("live IPv6 address mismatch")
		}
		return nil
	}
}
