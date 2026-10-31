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

// A live qemu:///system acceptance test for TCP, UDP, IPv4 and MAC rules,
// preserving the existing ICMP and reference functionality in the same filter.
func TestAccNWFilterResource_Protocols(t *testing.T) {
	var uuid string
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckNWFilterDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccNWFilterProtocolsConfig(443, "52:54:00:12:34:56"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("libvirt_nwfilter.test", "id"),
					resource.TestCheckResourceAttr("libvirt_nwfilter.test", "entries.#", "6"),
					testAccCheckNWFilterProtocols(&uuid, 443, "52:54:00:12:34:56"),
				),
			},
			{
				Config: testAccNWFilterProtocolsConfig(8443, "52:54:00:12:34:57"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("libvirt_nwfilter.test", "entries.0.rule.tcp.dst_port_start", "8443"),
					testAccCheckNWFilterProtocols(&uuid, 8443, "52:54:00:12:34:57"),
				),
			},
		},
	})
}

func testAccCheckNWFilterProtocols(uuid *string, tcpPort uint, macAddr string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources["libvirt_nwfilter.test"]
		if !ok {
			return fmt.Errorf("libvirt_nwfilter.test missing from state")
		}
		name, stateUUID := rs.Primary.Attributes["name"], rs.Primary.Attributes["id"]
		if name == "" || stateUUID == "" {
			return fmt.Errorf("missing name or UUID")
		}
		if *uuid == "" {
			*uuid = stateUUID
		} else if *uuid != stateUUID {
			return fmt.Errorf("UUID changed across update: %s -> %s", *uuid, stateUUID)
		}
		client, err := libvirtclient.NewClient(context.Background(), testAccLibvirtURI())
		if err != nil {
			return fmt.Errorf("connect libvirt: %w", err)
		}
		defer func() { _ = client.Close() }()
		filter, err := client.Libvirt().NwfilterLookupByName(name)
		if err != nil {
			return fmt.Errorf("lookup filter: %w", err)
		}
		if libvirtclient.UUIDString(filter.UUID) != stateUUID {
			return fmt.Errorf("live UUID mismatch")
		}
		xmlDoc, err := client.Libvirt().NwfilterGetXMLDesc(filter, 0)
		if err != nil {
			return fmt.Errorf("get live XML: %w", err)
		}
		var live libvirtxml.NWFilter
		if err := live.Unmarshal(xmlDoc); err != nil {
			return fmt.Errorf("parse live XML: %w", err)
		}
		if len(live.Entries) != 6 {
			return fmt.Errorf("expected 6 ordered entries, got %d", len(live.Entries))
		}
		for i := 0; i < 5; i++ {
			if live.Entries[i].Rule == nil {
				return fmt.Errorf("entry %d is not a rule", i)
			}
		}
		tcp := live.Entries[0].Rule.TCP
		if tcp == nil || tcp.DstPortStart.Uint == nil || *tcp.DstPortStart.Uint != tcpPort {
			return fmt.Errorf("incorrect TCP destination port: %#v", tcp)
		}
		udp := live.Entries[1].Rule.UDP
		if udp == nil || udp.DstPortStart.Uint == nil || *udp.DstPortStart.Uint != 53 {
			return fmt.Errorf("incorrect UDP rule: %#v", udp)
		}
		ip := live.Entries[2].Rule.IP
		if ip == nil || ip.SrcIPAddr.Str != "192.0.2.0" || ip.SrcIPMask.Str != "24" {
			return fmt.Errorf("incorrect IPv4 rule: %#v", ip)
		}
		mac := live.Entries[3].Rule.MAC
		if mac == nil || mac.SrcMACAddr.Str != macAddr {
			return fmt.Errorf("incorrect MAC rule: %#v", mac)
		}
		if live.Entries[4].Rule == nil || live.Entries[4].Rule.ICMP == nil {
			return fmt.Errorf("ICMP entry lost")
		}
		if live.Entries[5].Ref == nil || live.Entries[5].Ref.Filter != "allow-dhcp" {
			return fmt.Errorf("filter reference lost")
		}
		return nil
	}
}

func testAccNWFilterProtocolsConfig(port uint, mac string) string {
	return fmt.Sprintf(`
provider "libvirt" { uri = %[1]q }
resource "libvirt_nwfilter" "test" {
  name = "test-i2-nwfilter-protocols"
  entries = [
    {
      rule = {
        action = "accept"
        direction = "in"
        tcp = { src_ip = "192.0.2.10", dst_port_start = %[2]d }
      }
    },
    {
      rule = {
        action = "accept"
        direction = "in"
        udp = { dst_port_start = 53 }
      }
    },
    {
      rule = {
        action = "accept"
        direction = "in"
        ip = { src_ip = "192.0.2.0", src_ip_mask = "24", protocol = "tcp" }
      }
    },
    {
      rule = {
        action = "accept"
        direction = "in"
        mac = { src_mac = %[3]q }
      }
    },
    {
      rule = {
        action = "accept"
        direction = "in"
        icmp = { type = 8, code = 0 }
      }
    },
    { ref = { filter = "allow-dhcp" } }
  ]
}
`, testAccLibvirtURI(), port, mac)
}
