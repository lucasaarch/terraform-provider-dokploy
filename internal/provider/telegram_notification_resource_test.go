package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccTelegramNotificationResource(t *testing.T) {
	suffix := randInt()
	config := fmt.Sprintf(`
resource "dokploy_telegram_notification" "test" {
  name      = "tf-acc-telegram-%d"
  bot_token = "0:fake"
  chat_id   = "123"
}`, suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_telegram_notification.test", "id"),
					resource.TestCheckResourceAttr("dokploy_telegram_notification.test", "chat_id", "123"),
					resource.TestCheckResourceAttr("dokploy_telegram_notification.test", "app_deploy", "true"),
				),
			},
			{
				ResourceName:            "dokploy_telegram_notification.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"bot_token"},
			},
		},
	})
}
