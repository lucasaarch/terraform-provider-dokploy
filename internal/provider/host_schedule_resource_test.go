package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccHostScheduleResource(t *testing.T) {
	suffix := randInt()
	config := func(command string) string {
		return fmt.Sprintf(`
resource "dokploy_host_schedule" "test" {
  name            = "tf-acc-hs-%d"
  cron_expression = "0 0 * * *"
  command         = %q
  timezone        = "America/Sao_Paulo"
}`, suffix, command)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config("echo hi"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_host_schedule.test", "id"),
					resource.TestCheckResourceAttr("dokploy_host_schedule.test", "command", "echo hi"),
					resource.TestCheckResourceAttr("dokploy_host_schedule.test", "timezone", "America/Sao_Paulo"),
				),
			},
			{
				ResourceName:      "dokploy_host_schedule.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: config("echo updated"),
				Check:  resource.TestCheckResourceAttr("dokploy_host_schedule.test", "command", "echo updated"),
			},
		},
	})
}
