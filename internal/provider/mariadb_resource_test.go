package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccMariadbResource(t *testing.T) {
	suffix := randInt()
	config := func(image string) string {
		return fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-ma-proj-%d"
}

resource "dokploy_mariadb" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-ma"
  docker_image   = %q
  database_name  = "app"
  database_user  = "app"
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
				Config: config("mariadb:11"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_mariadb.test", "id"),
					resource.TestCheckResourceAttr("dokploy_mariadb.test", "status", "done"),
					resource.TestMatchResourceAttr("dokploy_mariadb.test", "database_password",
						regexp.MustCompile(`^[a-zA-Z0-9]{32}$`)),
					resource.TestMatchResourceAttr("dokploy_mariadb.test", "database_root_password",
						regexp.MustCompile(`^[a-zA-Z0-9]{32}$`)),
				),
			},
			{
				ResourceName:            "dokploy_mariadb.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeouts"},
			},
		},
	})
}
