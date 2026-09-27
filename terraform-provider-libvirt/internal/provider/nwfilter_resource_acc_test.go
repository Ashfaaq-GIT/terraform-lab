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

func testAccCheckNWFilterICMPExists(s *terraform.State) error {
	rs, ok := s.RootModule().Resources["libvirt_nwfilter.test"]
	if !ok {
		return fmt.Errorf("libvirt_nwfilter.test not found in Terraform state")
	}

	name := rs.Primary.Attributes["name"]
	if name == "" {
		return fmt.Errorf("network filter name is empty")
	}

	ctx := context.Background()

	client, err := libvirtclient.NewClient(ctx, testAccLibvirtURI())
	if err != nil {
		return fmt.Errorf("failed to create libvirt client: %w", err)
	}
	defer func() { _ = client.Close() }()

	filter, err := client.Libvirt().NwfilterLookupByName(name)
	if err != nil {
		return fmt.Errorf("failed to find network filter %q: %w", name, err)
	}

	xmlDoc, err := client.Libvirt().NwfilterGetXMLDesc(filter, 0)
	if err != nil {
		return fmt.Errorf("failed to read network filter XML: %w", err)
	}

	var liveFilter libvirtxml.NWFilter
	if err := liveFilter.Unmarshal(xmlDoc); err != nil {
		return fmt.Errorf("failed to parse network filter XML: %w", err)
	}

	if len(liveFilter.Entries) != 1 {
		return fmt.Errorf(
			"expected 1 network filter entry, got %d",
			len(liveFilter.Entries),
		)
	}

	rule := liveFilter.Entries[0].Rule
	if rule == nil {
		return fmt.Errorf("expected network filter rule")
	}

	if rule.Action != "accept" {
		return fmt.Errorf("expected action accept, got %q", rule.Action)
	}

	if rule.Direction != "in" {
		return fmt.Errorf("expected direction in, got %q", rule.Direction)
	}

	if rule.ICMP == nil {
		return fmt.Errorf("expected ICMP rule")
	}

	if rule.ICMP.Type.Uint == nil || *rule.ICMP.Type.Uint != 8 {
		return fmt.Errorf("expected ICMP type 8")
	}

	if rule.ICMP.Code.Uint == nil || *rule.ICMP.Code.Uint != 0 {
		return fmt.Errorf("expected ICMP code 0")
	}

	return nil
}

func testAccCheckNWFilterDestroy(s *terraform.State) error {
	ctx := context.Background()

	client, err := libvirtclient.NewClient(ctx, testAccLibvirtURI())
	if err != nil {
		return fmt.Errorf("failed to create libvirt client: %w", err)
	}
	defer func() { _ = client.Close() }()

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "libvirt_nwfilter" {
			continue
		}

		name := rs.Primary.Attributes["name"]
		if name == "" {
			continue
		}

		_, err := client.Libvirt().NwfilterLookupByName(name)
		if err == nil {
			return fmt.Errorf(
				"network filter %q still exists after destroy",
				name,
			)
		}
	}

	return nil
}

func TestAccNWFilterResource_ICMP(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckNWFilterDestroy,

		Steps: []resource.TestStep{
			{
				Config: testAccNWFilterResourceConfigICMP("test-nwfilter-icmp"),

				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr(
						"libvirt_nwfilter.test",
						"name",
						"test-nwfilter-icmp",
					),
					resource.TestCheckResourceAttrSet(
						"libvirt_nwfilter.test",
						"id",
					),
					resource.TestCheckResourceAttr(
						"libvirt_nwfilter.test",
						"entries.#",
						"1",
					),
					resource.TestCheckResourceAttr(
						"libvirt_nwfilter.test",
						"entries.0.rule.action",
						"accept",
					),
					resource.TestCheckResourceAttr(
						"libvirt_nwfilter.test",
						"entries.0.rule.direction",
						"in",
					),
					resource.TestCheckResourceAttr(
						"libvirt_nwfilter.test",
						"entries.0.rule.icmp.type",
						"8",
					),
					resource.TestCheckResourceAttr(
						"libvirt_nwfilter.test",
						"entries.0.rule.icmp.code",
						"0",
					),
					testAccCheckNWFilterICMPExists,
				),
			},
		},
	})
}

func testAccNWFilterResourceConfigICMP(name string) string {
	return fmt.Sprintf(`
provider "libvirt" {
  uri = %[1]q
}

resource "libvirt_nwfilter" "test" {
  name = %[2]q

  entries = [
    {
      rule = {
        action    = "accept"
        direction = "in"
        priority  = 100

        icmp = {
          type = 8
          code = 0
        }
      }
    }
  ]
}
`, testAccLibvirtURI(), name)
}
