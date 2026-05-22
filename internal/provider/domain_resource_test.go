package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDomainResource(t *testing.T) {
	suffix := randInt()
	host := fmt.Sprintf("tf-acc-%d.example.com", suffix)
	config := fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-domain-proj-%d"
}

resource "dokploy_application" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-domain-app"
  docker_image   = "nginx:1.27"
}

resource "dokploy_domain" "test" {
  application_id   = dokploy_application.test.id
  host             = %q
  port             = 80
  https            = false
  certificate_type = "none"
}`, suffix, host)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("dokploy_domain.test", "host", host),
					resource.TestCheckResourceAttr("dokploy_domain.test", "port", "80"),
					resource.TestCheckResourceAttrSet("dokploy_domain.test", "id"),
				),
			},
			{
				ResourceName:      "dokploy_domain.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}
