package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccNWFilterResource_Import(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckNWFilterDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccNWFilterResourceConfigICMP("test-nwfilter-i3-import"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("libvirt_nwfilter.test", "id"),
					resource.TestCheckResourceAttr(
						"libvirt_nwfilter.test", "name", "test-nwfilter-i3-import",
					),
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

// Verify import reconstructs all rule types delivered in Iteration 2.
func TestAccNWFilterResource_Import_Protocols(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckNWFilterDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccNWFilterProtocolsConfig(443, "52:54:00:12:34:56"),
			},
			{
				ResourceName:      "libvirt_nwfilter.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
