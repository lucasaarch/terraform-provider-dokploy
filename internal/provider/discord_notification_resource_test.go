package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDiscordNotificationResource(t *testing.T) {
	suffix := randInt()
	config := fmt.Sprintf(`
resource "dokploy_discord_notification" "test" {
  name        = "tf-acc-discord-%d"
  webhook_url = "https://discord.com/api/webhooks/0/Xfake"
}`, suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_discord_notification.test", "id"),
					resource.TestCheckResourceAttr("dokploy_discord_notification.test", "app_deploy", "true"),
				),
			},
			{
				ResourceName:            "dokploy_discord_notification.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"webhook_url"},
			},
		},
	})
}
