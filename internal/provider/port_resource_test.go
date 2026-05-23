package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPortResource(t *testing.T) {
	suffix := randInt()
	config := func(target int) string {
		return fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-port-proj-%d"
}

resource "dokploy_application" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-port-app"
  docker_image   = "nginx:alpine"
  timeouts { create = "15m" update = "15m" }
}

resource "dokploy_port" "test" {
  application_id  = dokploy_application.test.id
  published_port  = 8080
  target_port     = %d
}`, suffix, target)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(80),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_port.test", "id"),
					resource.TestCheckResourceAttr("dokploy_port.test", "published_port", "8080"),
					resource.TestCheckResourceAttr("dokploy_port.test", "target_port", "80"),
				),
			},
			{
				ResourceName:      "dokploy_port.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: config(8080),
				Check:  resource.TestCheckResourceAttr("dokploy_port.test", "target_port", "8080"),
			},
		},
	})
}
