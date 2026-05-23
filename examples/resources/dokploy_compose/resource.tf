resource "dokploy_compose" "monitoring" {
  environment_id = dokploy_project.obs.production_environment_id
  name           = "monitoring"

  compose_file = <<-EOT
    version: "3.8"
    services:
      prometheus:
        image: prom/prometheus:latest
        restart: unless-stopped
  EOT

  env = {
    PROM_PORT = "9090"
  }
}
