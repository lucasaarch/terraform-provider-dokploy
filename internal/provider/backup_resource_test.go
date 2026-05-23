package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// Helper config: project + postgres + destination + backup against postgres.
func backupPostgresConfig(suffix int, schedule string) string {
	return fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-bk-proj-%d"
}

resource "dokploy_postgres" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-bk-pg"
  docker_image   = "postgres:16"
  database_name  = "app"
  database_user  = "app"
  timeouts {
    create = "15m"
    update = "15m"
  }
}

resource "dokploy_destination" "test" {
  name              = "tf-acc-bk-dest-%d"
  provider_type     = "DigitalOcean"
  bucket            = "tf-acc-bucket-%d"
  endpoint          = "https://sfo3.digitaloceanspaces.com"
  access_key        = "AKIAEXAMPLEKEY1234"
  secret_access_key = "ExampleSecret1234567890abcdef"
}

resource "dokploy_backup" "test" {
  database_type  = "postgres"
  database_id    = dokploy_postgres.test.id
  destination_id = dokploy_destination.test.id
  schedule       = %q
  prefix         = "tf-acc/postgres/"
}`, suffix, suffix, suffix, schedule)
}

func TestAccBackupResource(t *testing.T) {
	suffix := randInt()
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: backupPostgresConfig(suffix, "0 3 * * *"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_backup.test", "id"),
					resource.TestCheckResourceAttr("dokploy_backup.test", "database_type", "postgres"),
					resource.TestCheckResourceAttr("dokploy_backup.test", "schedule", "0 3 * * *"),
				),
			},
			{
				ResourceName:      "dokploy_backup.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: backupPostgresConfig(suffix, "0 4 * * *"),
				Check:  resource.TestCheckResourceAttr("dokploy_backup.test", "schedule", "0 4 * * *"),
			},
		},
	})
}

// TestAccBackup_WebServer verifies that web-server database_type produces a
// clear error. The application.one endpoint does not return backups[], making
// post-create ID discovery impossible. This is a known provider limitation.
func TestAccBackup_WebServer(t *testing.T) {
	suffix := randInt()
	config := fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-bk-app-proj-%d"
}

resource "dokploy_application" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-bk-app"
  docker_image   = "nginx:1.27"
  timeouts {
    create = "15m"
    update = "15m"
  }
}

resource "dokploy_destination" "test" {
  name              = "tf-acc-bk-app-dest-%d"
  provider_type     = "DigitalOcean"
  bucket            = "tf-acc-bucket-%d"
  endpoint          = "https://sfo3.digitaloceanspaces.com"
  access_key        = "AKIAEXAMPLEKEY1234"
  secret_access_key = "ExampleSecret1234567890abcdef"
}

resource "dokploy_backup" "test" {
  database_type  = "web-server"
  database_id    = dokploy_application.test.id
  destination_id = dokploy_destination.test.id
  schedule       = "0 5 * * 0"
  prefix         = "tf-acc/web-server/"
}`, suffix, suffix, suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config:      config,
				ExpectError: regexp.MustCompile(`web-server`),
			},
		},
	})
}
