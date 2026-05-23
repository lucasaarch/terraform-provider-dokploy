resource "dokploy_discord_notification" "alerts" {
  name        = "production-alerts"
  webhook_url = var.discord_webhook
  decoration  = true
}
