package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccGotifyNotificationResource(t *testing.T) {
	suffix := randInt()
	config := fmt.Sprintf(`
resource "dokploy_gotify_notification" "test" {
  name       = "tf-acc-gotify-%d"
  server_url = "https://gotify.example.com"
  app_token  = "Afake"
  priority   = 5
}`, suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_gotify_notification.test", "id"),
					resource.TestCheckResourceAttr("dokploy_gotify_notification.test", "server_url", "https://gotify.example.com"),
					resource.TestCheckResourceAttr("dokploy_gotify_notification.test", "priority", "5"),
					resource.TestCheckResourceAttr("dokploy_gotify_notification.test", "app_deploy", "true"),
				),
			},
			{
				ResourceName:            "dokploy_gotify_notification.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"app_token"},
			},
		},
	})
}
