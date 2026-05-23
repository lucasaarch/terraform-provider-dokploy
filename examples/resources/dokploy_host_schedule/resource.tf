resource "dokploy_host_schedule" "rotate_logs" {
  name            = "rotate-traefik-logs"
  cron_expression = "0 0 * * *"
  command         = "find /var/log/dokploy -name '*.log.*' -mtime +14 -delete"
  timezone        = "America/Sao_Paulo"
}
