resource "dokploy_mysql" "db" {
  environment_id = dokploy_project.app.production_environment_id
  name           = "app-db"
  docker_image   = "mysql:8"
  database_name  = "app"
  database_user  = "app"
  # database_password and database_root_password omitted → provider generates.
}
