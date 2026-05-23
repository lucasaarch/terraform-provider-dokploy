package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccSlackNotificationResource(t *testing.T) {
	suffix := randInt()
	config := fmt.Sprintf(`
resource "dokploy_slack_notification" "test" {
  name        = "tf-acc-slack-%d"
  webhook_url = "https://hooks.slack.com/services/T0/B0/Xfake"
  channel     = "#tf-acc-tests"
}`, suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_slack_notification.test", "id"),
					resource.TestCheckResourceAttr("dokploy_slack_notification.test", "channel", "#tf-acc-tests"),
					resource.TestCheckResourceAttr("dokploy_slack_notification.test", "app_deploy", "true"),
				),
			},
			{
				ResourceName:            "dokploy_slack_notification.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"webhook_url"},
			},
		},
	})
}
