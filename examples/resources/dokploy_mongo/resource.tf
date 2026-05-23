resource "dokploy_mongo" "db" {
  environment_id = dokploy_project.app.production_environment_id
  name           = "app-db"
  docker_image   = "mongo:7"
  database_user  = "root"
}
