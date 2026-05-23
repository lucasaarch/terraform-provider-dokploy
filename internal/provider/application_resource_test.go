package provider

import (
	"fmt"
	"os"
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

func TestAccApplicationResource_OnServer(t *testing.T) {
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
data "dokploy_organization" "current" { name = %q }
resource "dokploy_ssh_key" "test" {
  organization_id = data.dokploy_organization.current.id
  name            = "tf-acc-app-srv-key-%d"
  private_key     = %q
  public_key      = %q
}
resource "dokploy_server" "test" {
  name        = "tf-acc-app-srv-%d"
  ip_address  = %q
  port        = %s
  username    = %q
  ssh_key_id  = dokploy_ssh_key.test.id
}
resource "dokploy_project" "test" {
  name = "tf-acc-app-srv-proj-%d"
}
resource "dokploy_application" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-app-on-server"
  docker_image   = "nginx:1.27"
  server_id      = dokploy_server.test.id
  timeouts { create = "15m" update = "15m" }
}`, firstOrgName(t), suffix, priv, pub, suffix, ip, port, user, suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_application.test", "id"),
					resource.TestCheckResourceAttrPair("dokploy_application.test", "server_id", "dokploy_server.test", "id"),
				),
			},
		},
	})
}
