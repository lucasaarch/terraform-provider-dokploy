resource "dokploy_application_schedule" "warmup" {
  application_id  = dokploy_application.api.id
  name            = "warmup-cache"
  cron_expression = "*/15 * * * *"
  command         = "curl -s http://localhost:3000/internal/warmup"
}
