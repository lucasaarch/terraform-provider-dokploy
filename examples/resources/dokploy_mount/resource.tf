resource "dokploy_mount" "static" {
  service_id = dokploy_application.web.id
  type       = "bind"
  mount_path = "/srv/static"
  host_path  = "/var/www/static"
}

resource "dokploy_mount" "data" {
  service_id  = dokploy_postgres.db.id
  type        = "volume"
  mount_path  = "/var/lib/postgresql/data"
  volume_name = "pg-data"
}

resource "dokploy_mount" "config" {
  service_id = dokploy_application.web.id
  type       = "file"
  mount_path = "/etc/nginx/conf.d/extra.conf"
  content    = "client_max_body_size 100M;\n"
}
