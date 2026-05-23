package provider

import (
	"fmt"
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
