package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccServerScheduleResource(t *testing.T) {
	if os.Getenv("DOKPLOY_TEST_SERVER_IP") == "" {
		t.Skip("set DOKPLOY_TEST_SERVER_IP (and friends) to run.")
	}
	ip := os.Getenv("DOKPLOY_TEST_SERVER_IP")
	user := os.Getenv("DOKPLOY_TEST_SERVER_USER")
	port := os.Getenv("DOKPLOY_TEST_SERVER_PORT")
	priv := os.Getenv("DOKPLOY_TEST_SERVER_PRIVATE_KEY")
	pub := os.Getenv("DOKPLOY_TEST_SERVER_PUBLIC_KEY")

	suffix := randInt()
	config := fmt.Sprintf(`
data "dokploy_organization" "current" {
  name = %q
}

resource "dokploy_ssh_key" "test" {
  organization_id = data.dokploy_organization.current.id
  name            = "tf-acc-ssched-key-%d"
  private_key     = %q
  public_key      = %q
}

resource "dokploy_server" "test" {
  name        = "tf-acc-ssched-srv-%d"
  ip_address  = %q
  port        = %s
  username    = %q
  ssh_key_id  = dokploy_ssh_key.test.id
}

resource "dokploy_server_schedule" "test" {
  server_id       = dokploy_server.test.id
  name            = "tf-acc-server-sched-%d"
  cron_expression = "0 5 * * *"
  command         = "echo hi"
}`, firstOrgName(t), suffix, priv, pub, suffix, ip, port, user, suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_server_schedule.test", "id"),
					resource.TestCheckResourceAttrSet("dokploy_server_schedule.test", "app_name"),
				),
			},
			{
				ResourceName:      "dokploy_server_schedule.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
