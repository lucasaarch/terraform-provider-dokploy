package provider

import (
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccServerResource is opt-in: it requires a reachable VM whose
// authorized_keys contains the public key the provider generates. Set the env
// vars below to enable.
func TestAccServerResource(t *testing.T) {
	ip := os.Getenv("DOKPLOY_TEST_SERVER_IP")
	if ip == "" {
		t.Skip("set DOKPLOY_TEST_SERVER_IP, DOKPLOY_TEST_SERVER_USER, DOKPLOY_TEST_SERVER_PORT, DOKPLOY_TEST_SERVER_PRIVATE_KEY, and DOKPLOY_TEST_SERVER_PUBLIC_KEY to run.")
	}
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
  name            = "tf-acc-srv-key-%d"
  private_key     = %q
  public_key      = %q
}

resource "dokploy_server" "test" {
  name        = "tf-acc-srv-%d"
  description = "acc test"
  ip_address  = %q
  port        = %s
  username    = %q
  ssh_key_id  = dokploy_ssh_key.test.id
  server_type = "deploy"
}`, firstOrgName(t), suffix, priv, pub, suffix, ip, port, user)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_server.test", "id"),
					resource.TestCheckResourceAttr("dokploy_server.test", "server_type", "deploy"),
				),
			},
			{
				ResourceName:      "dokploy_server.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
