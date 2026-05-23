resource "dokploy_slack_notification" "alerts" {
  name        = "production-alerts"
  webhook_url = var.slack_webhook
  channel     = "#deploys"
}
