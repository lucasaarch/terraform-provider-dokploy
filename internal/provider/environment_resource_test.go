package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccEnvironmentResource(t *testing.T) {
	suffix := randInt()
	config := func(level string) string {
		return fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-env-proj-%d"
}

resource "dokploy_environment" "test" {
  project_id  = dokploy_project.test.id
  name        = "staging"
  description = "acc test environment"
  env = {
    LOG_LEVEL = %q
  }
}`, suffix, level)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("debug"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dokploy_environment.test", "name", "staging"),
					resource.TestCheckResourceAttrSet("dokploy_environment.test", "id"),
					resource.TestCheckResourceAttr("dokploy_environment.test", "env.LOG_LEVEL", "debug"),
				),
			},
			{
				ResourceName:      "dokploy_environment.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: config("warn"),
				Check:  resource.TestCheckResourceAttr("dokploy_environment.test", "env.LOG_LEVEL", "warn"),
			},
		},
	})
}
