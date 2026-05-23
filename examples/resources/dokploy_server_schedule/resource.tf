resource "dokploy_server_schedule" "vacuum" {
  server_id       = dokploy_server.worker_sp.id
  name            = "pg-vacuum-weekly"
  cron_expression = "0 4 * * 0"
  command         = "docker exec ${dokploy_postgres.db.app_name} psql -U app -c 'VACUUM ANALYZE'"
  timezone        = "America/Sao_Paulo"
}
