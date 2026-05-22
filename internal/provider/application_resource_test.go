package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccApplicationResource(t *testing.T) {
	suffix := randInt()
	config := func(image string) string {
		return fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-app-proj-%d"
}

resource "dokploy_application" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-app"
  docker_image   = %q
  env = {
    APP_ENV = "test"
  }
  timeouts {
    create = "15m"
    update = "15m"
  }
}`, suffix, image)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("nginx:1.27"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dokploy_application.test", "docker_image", "nginx:1.27"),
					resource.TestCheckResourceAttrSet("dokploy_application.test", "id"),
					resource.TestCheckResourceAttrSet("dokploy_application.test", "app_name"),
					resource.TestCheckResourceAttr("dokploy_application.test", "status", "done"),
				),
			},
			{
				ResourceName:            "dokploy_application.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"registry_password", "timeouts"},
			},
			{
				Config: config("nginx:1.28"),
				Check:  resource.TestCheckResourceAttr("dokploy_application.test", "docker_image", "nginx:1.28"),
			},
		},
	})
}
