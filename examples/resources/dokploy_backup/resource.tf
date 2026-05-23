# Backup of a managed Postgres database.
# Supported database_type values: postgres, mysql, mariadb, mongo.
# (web-server backups are not supported in this provider version because the
# Dokploy API does not list them on application.one.)
resource "dokploy_backup" "db_daily" {
  database_type  = "postgres"
  database_id    = dokploy_postgres.db.id
  destination_id = dokploy_destination.s3.id
  schedule       = "0 3 * * *"
  prefix         = "postgres/app/"
}
