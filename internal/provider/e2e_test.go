package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAccEndToEnd builds the full resource graph in one apply:
// organization (data source) -> project -> environment + application -> domain.
func TestAccEndToEnd(t *testing.T) {
	suffix := randInt()
	config := fmt.Sprintf(`
data "dokploy_organization" "e2e" {
  name = %q
}

resource "dokploy_project" "e2e" {
  name           = "tf-acc-e2e-proj-%d"
  production_env = { LOG_LEVEL = "info" }
}

resource "dokploy_environment" "e2e" {
  project_id = dokploy_project.e2e.id
  name       = "staging"
  env        = { LOG_LEVEL = "debug" }
}

resource "dokploy_application" "e2e" {
  environment_id = dokploy_project.e2e.production_environment_id
  name           = "tf-acc-e2e-app"
  docker_image   = "nginx:1.27"
  timeouts { create = "15m" }
}

resource "dokploy_domain" "e2e" {
  application_id = dokploy_application.e2e.id
  host           = "tf-acc-e2e-%d.example.com"
  port           = 80
}`, firstOrgName(t), suffix, suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("data.dokploy_organization.e2e", "id"),
					resource.TestCheckResourceAttrSet("dokploy_project.e2e", "id"),
					// the project's org must match the data source's org
					resource.TestCheckResourceAttrPair(
						"dokploy_project.e2e", "organization_id",
						"data.dokploy_organization.e2e", "id"),
					resource.TestCheckResourceAttrSet("dokploy_environment.e2e", "id"),
					resource.TestCheckResourceAttr("dokploy_application.e2e", "status", "done"),
					resource.TestCheckResourceAttrSet("dokploy_domain.e2e", "id"),
				),
			},
		},
	})
}
