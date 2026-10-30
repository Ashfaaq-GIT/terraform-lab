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

// TestAccNWFilterResource_Update verifies that an existing
// libvirt network filter can be updated without changing
// its UUID or replacing the Terraform resource.
func TestAccNWFilterResource_Update(t *testing.T) {
	var originalUUID string

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckNWFilterDestroy,

		Steps: []resource.TestStep{
			{
				Config: testAccNWFilterUpdateConfig(
					"test-i2-nwfilter-update",
					"accept",
					8,
					100,
				),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"libvirt_nwfilter.test",
						"name",
						"test-i2-nwfilter-update",
					),
					resource.TestCheckResourceAttrSet(
						"libvirt_nwfilter.test",
						"id",
					),
					testAccCheckNWFilterUpdate(
						&originalUUID,
						"accept",
						8,
						100,
					),
				),
			},
			{
				Config: testAccNWFilterUpdateConfig(
					"test-i2-nwfilter-update",
					"drop",
					0,
					110,
				),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(
						"libvirt_nwfilter.test",
						"entries.0.rule.action",
						"drop",
					),
					resource.TestCheckResourceAttr(
						"libvirt_nwfilter.test",
						"entries.0.rule.icmp.type",
						"0",
					),
					testAccCheckNWFilterUpdate(
						&originalUUID,
						"drop",
						0,
						110,
					),
				),
			},
		},
	})
}

// testAccCheckNWFilterUpdate verifies the actual
// libvirt XML and checks that the UUID is unchanged.
func testAccCheckNWFilterUpdate(
	originalUUID *string,
	expectedAction string,
	expectedType uint,
	expectedPriority int,
) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources["libvirt_nwfilter.test"]
		if !ok {
			return fmt.Errorf(
				"network filter not found in Terraform state",
			)
		}

		name := rs.Primary.Attributes["name"]
		stateUUID := rs.Primary.Attributes["id"]

		if name == "" || stateUUID == "" {
			return fmt.Errorf(
				"network filter name or UUID is empty",
			)
		}

		if *originalUUID == "" {
			*originalUUID = stateUUID
		} else if *originalUUID != stateUUID {
			return fmt.Errorf(
				"UUID changed from %s to %s",
				*originalUUID,
				stateUUID,
			)
		}

		ctx := context.Background()

		client, err := libvirtclient.NewClient(
			ctx,
			testAccLibvirtURI(),
		)
		if err != nil {
			return fmt.Errorf(
				"failed to connect to libvirt: %w",
				err,
			)
		}
		defer func() { _ = client.Close() }()

		filter, err := client.Libvirt().NwfilterLookupByName(name)
		if err != nil {
			return fmt.Errorf(
				"failed to find filter %q: %w",
				name,
				err,
			)
		}

		liveUUID := libvirtclient.UUIDString(filter.UUID)
		if liveUUID != stateUUID {
			return fmt.Errorf(
				"live UUID %s differs from Terraform UUID %s",
				liveUUID,
				stateUUID,
			)
		}

		xmlDoc, err := client.Libvirt().NwfilterGetXMLDesc(
			filter,
			0,
		)
		if err != nil {
			return fmt.Errorf(
				"failed to retrieve live XML: %w",
				err,
			)
		}

		var liveFilter libvirtxml.NWFilter
		if err := liveFilter.Unmarshal(xmlDoc); err != nil {
			return fmt.Errorf(
				"failed to parse live XML: %w",
				err,
			)
		}

		if len(liveFilter.Entries) != 1 {
			return fmt.Errorf(
				"expected one filter entry, got %d",
				len(liveFilter.Entries),
			)
		}

		rule := liveFilter.Entries[0].Rule
		if rule == nil {
			return fmt.Errorf("expected a filter rule")
		}

		if rule.Action != expectedAction {
			return fmt.Errorf(
				"expected action %q, got %q",
				expectedAction,
				rule.Action,
			)
		}

		if rule.Priority != expectedPriority {
			return fmt.Errorf(
				"expected rule priority %d, got %d",
				expectedPriority,
				rule.Priority,
			)
		}

		if rule.ICMP == nil {
			return fmt.Errorf(
				"expected ICMP criteria in live XML",
			)
		}

		if rule.ICMP.Type.Uint == nil ||
			*rule.ICMP.Type.Uint != expectedType {
			return fmt.Errorf(
				"expected ICMP type %d in live XML",
				expectedType,
			)
		}

		if rule.ICMP.Code.Uint == nil ||
			*rule.ICMP.Code.Uint != 0 {
			return fmt.Errorf(
				"expected ICMP code 0 in live XML",
			)
		}

		return nil
	}
}

// Generates Terraform configuration for the
// create and update test steps.
func testAccNWFilterUpdateConfig(
	name string,
	action string,
	icmpType int,
	priority int,
) string {
	return fmt.Sprintf(`
provider "libvirt" {
  uri = %q
}

resource "libvirt_nwfilter" "test" {
  name = %q

  entries = [
    {
      rule = {
        action    = %q
        direction = "in"
        priority  = %d

        icmp = {
          type = %d
          code = 0
        }
      }
    }
  ]
}
`,
		testAccLibvirtURI(),
		name,
		action,
		priority,
		icmpType,
	)
}
