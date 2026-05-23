resource "dokploy_postgres" "db" {
  environment_id = dokploy_project.app.production_environment_id
  name           = "app-db"
  docker_image   = "postgres:16"
  database_name  = "app"
  database_user  = "app"
  # database_password omitted → provider generates a 32-char random.
}
