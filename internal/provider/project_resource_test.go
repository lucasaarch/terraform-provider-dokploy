package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccProjectResource(t *testing.T) {
	name := fmt.Sprintf("tf-acc-proj-%d", randInt())
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
resource "dokploy_project" "test" {
  name        = %q
  description = "created by acceptance test"
  production_env = {
    LOG_LEVEL = "info"
  }
}`, name),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dokploy_project.test", "name", name),
					resource.TestCheckResourceAttrSet("dokploy_project.test", "id"),
					resource.TestCheckResourceAttrSet("dokploy_project.test", "production_environment_id"),
					resource.TestCheckResourceAttr("dokploy_project.test", "production_env.LOG_LEVEL", "info"),
				),
			},
			{
				ResourceName:            "dokploy_project.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"production_env"},
			},
		},
	})
}
