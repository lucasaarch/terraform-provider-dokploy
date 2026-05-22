resource "dokploy_project" "app" {
  name        = "my-app"
  description = "Main application project"

  production_env = {
    LOG_LEVEL = "info"
  }
}
