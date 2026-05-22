resource "dokploy_domain" "web" {
  application_id   = dokploy_application.api.id
  host             = "api.example.com"
  port             = 8080
  https            = true
  certificate_type = "letsencrypt"
}
