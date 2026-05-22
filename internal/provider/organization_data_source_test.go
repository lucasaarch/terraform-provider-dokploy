package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccOrganizationDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				// name is resolved at runtime so the test does not hardcode
				// an instance-specific organization name.
				Config: fmt.Sprintf(`data "dokploy_organization" "current" { name = %q }`, firstOrgName(t)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.dokploy_organization.current", "id"),
					resource.TestCheckResourceAttrSet("data.dokploy_organization.current", "name"),
				),
			},
		},
	})
}
