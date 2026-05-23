resource "dokploy_email_notification" "alerts" {
  name         = "ops-team"
  smtp_server  = "smtp.example.com"
  smtp_port    = 587
  username     = var.smtp_user
  password     = var.smtp_password
  from_address = "alerts@example.com"
  to_addresses = ["ops@example.com"]
}
