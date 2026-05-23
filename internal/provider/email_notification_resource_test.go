package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccEmailNotificationResource(t *testing.T) {
	suffix := randInt()
	config := fmt.Sprintf(`
resource "dokploy_email_notification" "test" {
  name         = "tf-acc-email-%d"
  smtp_server  = "smtp.example.com"
  smtp_port    = 587
  username     = "user@example.com"
  password     = "fakepassword"
  from_address = "alerts@example.com"
  to_addresses = ["ops@example.com"]
}`, suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_email_notification.test", "id"),
					resource.TestCheckResourceAttr("dokploy_email_notification.test", "smtp_server", "smtp.example.com"),
					resource.TestCheckResourceAttr("dokploy_email_notification.test", "app_deploy", "true"),
				),
			},
			{
				ResourceName:            "dokploy_email_notification.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"password"},
			},
		},
	})
}
