resource "dokploy_port" "metrics" {
  application_id = dokploy_application.api.id
  published_port = 9090
  target_port    = 9090
}
