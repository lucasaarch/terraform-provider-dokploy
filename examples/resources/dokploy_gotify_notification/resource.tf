resource "dokploy_gotify_notification" "alerts" {
  name       = "gotify-ops"
  server_url = "https://gotify.example.com"
  app_token  = var.gotify_app_token
  priority   = 5
}
