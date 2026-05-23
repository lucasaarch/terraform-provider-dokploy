package provider

import (
	"fmt"
	"os"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccPostgresResource(t *testing.T) {
	suffix := randInt()
	config := func(image string) string {
		return fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-pg-proj-%d"
}

resource "dokploy_postgres" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-pg"
  docker_image   = %q
  database_name  = "app"
  database_user  = "app"
  # database_password omitted on purpose: provider must generate.
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
				Config: config("postgres:16"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_postgres.test", "id"),
					resource.TestCheckResourceAttrSet("dokploy_postgres.test", "app_name"),
					resource.TestCheckResourceAttr("dokploy_postgres.test", "status", "done"),
					resource.TestMatchResourceAttr("dokploy_postgres.test", "database_password",
						regexp.MustCompile(`^[a-zA-Z0-9]{32}$`)),
				),
			},
			{
				ResourceName:            "dokploy_postgres.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeouts"},
			},
			{
				Config: config("postgres:17"),
				Check:  resource.TestCheckResourceAttr("dokploy_postgres.test", "docker_image", "postgres:17"),
			},
		},
	})
}

func TestAccPostgresResource_OnServer(t *testing.T) {
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
  name            = "tf-acc-pg-srv-key-%d"
  private_key     = %q
  public_key      = %q
}
resource "dokploy_server" "test" {
  name        = "tf-acc-pg-srv-%d"
  ip_address  = %q
  port        = %s
  username    = %q
  ssh_key_id  = dokploy_ssh_key.test.id
}
resource "dokploy_project" "test" {
  name = "tf-acc-pg-srv-proj-%d"
}
resource "dokploy_postgres" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-pg-on-server"
  docker_image   = "postgres:16"
  database_name  = "app"
  database_user  = "app"
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
					resource.TestCheckResourceAttrSet("dokploy_postgres.test", "id"),
					resource.TestCheckResourceAttrPair("dokploy_postgres.test", "server_id", "dokploy_server.test", "id"),
				),
			},
		},
	})
}
