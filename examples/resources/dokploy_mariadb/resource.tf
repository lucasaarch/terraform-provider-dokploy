resource "dokploy_mariadb" "db" {
  environment_id = dokploy_project.app.production_environment_id
  name           = "app-db"
  docker_image   = "mariadb:11"
  database_name  = "app"
  database_user  = "app"
}
