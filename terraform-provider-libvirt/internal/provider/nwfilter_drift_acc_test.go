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

func TestAccNWFilterResource_DriftRepair(t *testing.T) {
	const name = "test-nwfilter-i3-drift"
	config := testAccNWFilterResourceConfigICMP(name)
	var originalUUID string

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAccCheckNWFilterDestroy,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckNWFilterICMPExists,
					i3DriftCheckUUID(&originalUUID),
				),
			},
			{
				PreConfig: func() {
					i3DriftChangeRuleOutsideTerraform(t, name)
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					testAccCheckNWFilterICMPExists,
					i3DriftCheckUUID(&originalUUID),
				),
			},
			{
				Config:   config,
				PlanOnly: true,
			},
		},
	})
}

func i3DriftCheckUUID(original *string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources["libvirt_nwfilter.test"]
		if !ok {
			return fmt.Errorf("NWFilter missing from Terraform state")
		}
		uuid := rs.Primary.Attributes["id"]
		if uuid == "" {
			return fmt.Errorf("NWFilter UUID is empty")
		}
		if *original == "" {
			*original = uuid
		} else if uuid != *original {
			return fmt.Errorf("UUID changed from %s to %s", *original, uuid)
		}
		return nil
	}
}

func i3DriftChangeRuleOutsideTerraform(t *testing.T, name string) {
	t.Helper()

	client, err := libvirtclient.NewClient(context.Background(), testAccLibvirtURI())
	if err != nil {
		t.Fatalf("connect to libvirt: %v", err)
	}
	defer func() { _ = client.Close() }()

	filter, err := client.Libvirt().NwfilterLookupByName(name)
	if err != nil {
		t.Fatalf("find test filter: %v", err)
	}

	xmlDoc, err := client.Libvirt().NwfilterGetXMLDesc(filter, 0)
	if err != nil {
		t.Fatalf("read filter XML: %v", err)
	}

	var live libvirtxml.NWFilter
	if err := live.Unmarshal(xmlDoc); err != nil {
		t.Fatalf("parse filter XML: %v", err)
	}
	if len(live.Entries) != 1 ||
		live.Entries[0].Rule == nil ||
		live.Entries[0].Rule.ICMP == nil {
		t.Fatal("expected one ICMP rule")
	}

	live.Entries[0].Rule.Action = "drop"

	changedXML, err := live.Marshal()
	if err != nil {
		t.Fatalf("generate modified XML: %v", err)
	}

	updated, err := client.Libvirt().NwfilterDefineXML(changedXML)
	if err != nil {
		t.Fatalf("modify filter directly in libvirt: %v", err)
	}
	if updated.UUID != filter.UUID {
		t.Fatal("external change unexpectedly modified the filter UUID")
	}

	t.Log("Changed ICMP action directly in libvirt: accept -> drop")
}
