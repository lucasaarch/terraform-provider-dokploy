package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccMountResource_Bind(t *testing.T) {
	suffix := randInt()
	config := fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-mount-bind-proj-%d"
}

resource "dokploy_application" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-mount-bind-app"
  docker_image   = "nginx:alpine"
  timeouts {
    create = "15m"
    update = "15m"
  }
}

resource "dokploy_mount" "test" {
  service_id = dokploy_application.test.id
  type       = "bind"
  mount_path = "/srv/static"
  host_path  = "/var/www/static"
}`, suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_mount.test", "id"),
					resource.TestCheckResourceAttr("dokploy_mount.test", "type", "bind"),
					resource.TestCheckResourceAttr("dokploy_mount.test", "host_path", "/var/www/static"),
				),
			},
			{
				ResourceName:            "dokploy_mount.test",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"service_id"},
			},
		},
	})
}

func TestAccMountResource_Volume(t *testing.T) {
	suffix := randInt()
	config := fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-mount-vol-proj-%d"
}

resource "dokploy_application" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-mount-vol-app"
  docker_image   = "nginx:alpine"
  timeouts {
    create = "15m"
    update = "15m"
  }
}

resource "dokploy_mount" "test" {
  service_id  = dokploy_application.test.id
  type        = "volume"
  mount_path  = "/data"
  volume_name = "tf-acc-volume-%d"
}`, suffix, suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_mount.test", "id"),
					resource.TestCheckResourceAttr("dokploy_mount.test", "type", "volume"),
				),
			},
		},
	})
}

func TestAccMountResource_File(t *testing.T) {
	suffix := randInt()
	config := fmt.Sprintf(`
resource "dokploy_project" "test" {
  name = "tf-acc-mount-file-proj-%d"
}

resource "dokploy_application" "test" {
  environment_id = dokploy_project.test.production_environment_id
  name           = "tf-acc-mount-file-app"
  docker_image   = "nginx:alpine"
  timeouts {
    create = "15m"
    update = "15m"
  }
}

resource "dokploy_mount" "test" {
  service_id = dokploy_application.test.id
  type       = "file"
  mount_path = "/etc/nginx/conf.d/extra.conf"
  content    = "client_max_body_size 100M;\n"
}`, suffix)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_mount.test", "id"),
					resource.TestCheckResourceAttr("dokploy_mount.test", "type", "file"),
				),
			},
		},
	})
}
