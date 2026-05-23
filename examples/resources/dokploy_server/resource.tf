resource "dokploy_server" "worker_sp" {
  name        = "worker-sp"
  description = "Worker São Paulo"
  ip_address  = "203.0.113.10"
  port        = 22
  username    = "dokploy"
  ssh_key_id  = dokploy_ssh_key.worker.id
  server_type = "deploy"
}
