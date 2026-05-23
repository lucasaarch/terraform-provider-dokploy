package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRedisResource(t *testing.T) {
	suffix := randInt()
	config := func(image string) string {
		return fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-re-proj-%d"
}

resource "dokploy_redis" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-re"
  docker_image   = %q
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
				Config: config("redis:7.2"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_redis.test", "id"),
					resource.TestCheckResourceAttr("dokploy_redis.test", "status", "done"),
					resource.TestMatchResourceAttr("dokploy_redis.test", "database_password",
						regexp.MustCompile(`^[a-zA-Z0-9]{32}$`)),
				),
			},
			{
				ResourceName:            "dokploy_redis.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"timeouts"},
			},
		},
	})
}
