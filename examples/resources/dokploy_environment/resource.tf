resource "dokploy_environment" "staging" {
  project_id  = dokploy_project.app.id
  name        = "staging"
  description = "Staging environment"

  env = {
    LOG_LEVEL = "debug"
  }
}
