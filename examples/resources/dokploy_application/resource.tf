resource "dokploy_application" "api" {
  environment_id = dokploy_project.app.production_environment_id
  name           = "api"
  docker_image   = "nginx:1.27"

  env = {
    PORT = "8080"
  }

  timeouts {
    create = "15m"
    update = "15m"
  }
}
