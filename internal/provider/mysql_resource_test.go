package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccMysqlResource(t *testing.T) {
	suffix := randInt()
	config := func(image string) string {
		return fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-my-proj-%d"
}

resource "dokploy_mysql" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-my"
  docker_image   = %q
  database_name  = "app"
  database_user  = "app"
  # database_password + database_root_password omitted on purpose.
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
				Config: config("mysql:8"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_mysql.test", "id"),
					resource.TestCheckResourceAttr("dokploy_mysql.test", "status", "done"),
					resource.TestMatchResourceAttr("dokploy_mysql.test", "database_password",
						regexp.MustCompile(`^[a-zA-Z0-9]{32}$`)),
					resource.TestMatchResourceAttr("dokploy_mysql.test", "database_root_password",
						regexp.MustCompile(`^[a-zA-Z0-9]{32}$`)),
				),
			},
			{
				ResourceName:            "dokploy_mysql.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeouts"},
			},
		},
	})
}
