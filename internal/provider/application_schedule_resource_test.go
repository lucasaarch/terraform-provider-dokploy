package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccApplicationScheduleResource(t *testing.T) {
	suffix := randInt()
	config := func(command string) string {
		return fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-as-proj-%d"
}

resource "dokploy_application" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-as-app"
  docker_image   = "nginx:1.27"
  timeouts {
    create = "15m"
    update = "15m"
  }
}

resource "dokploy_application_schedule" "test" {
  application_id  = dokploy_application.test.id
  name            = "tf-acc-as-sched"
  cron_expression = "0 4 * * *"
  command         = %q
}`, suffix, command)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("echo hello"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_application_schedule.test", "id"),
					resource.TestCheckResourceAttrSet("dokploy_application_schedule.test", "app_name"),
					resource.TestCheckResourceAttr("dokploy_application_schedule.test", "command", "echo hello"),
				),
			},
			{
				ResourceName:      "dokploy_application_schedule.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: config("echo updated"),
				Check:  resource.TestCheckResourceAttr("dokploy_application_schedule.test", "command", "echo updated"),
			},
		},
	})
}
