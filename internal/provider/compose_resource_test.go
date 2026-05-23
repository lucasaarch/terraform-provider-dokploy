package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccComposeResource(t *testing.T) {
	suffix := randInt()
	composeYAML := `version: "3"
services:
  hello:
    image: nginx:alpine
    restart: unless-stopped
`
	config := func(env string) string {
		return fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-compose-proj-%d"
}

resource "dokploy_compose" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-compose"
  compose_file   = %q
  env = {
    HELLO = %q
  }
  timeouts {
    create = "15m"
    update = "15m"
  }
}`, suffix, composeYAML, env)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("v1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_compose.test", "id"),
					resource.TestCheckResourceAttrSet("dokploy_compose.test", "app_name"),
					resource.TestCheckResourceAttr("dokploy_compose.test", "status", "done"),
				),
			},
			{
				ResourceName:            "dokploy_compose.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeouts"},
			},
			{
				Config: config("v2"),
				Check:  resource.TestCheckResourceAttr("dokploy_compose.test", "env.HELLO", "v2"),
			},
		},
	})
}
