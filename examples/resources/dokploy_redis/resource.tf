resource "dokploy_redis" "cache" {
  environment_id = dokploy_project.app.production_environment_id
  name           = "app-cache"
  docker_image   = "redis:7.2"
}
