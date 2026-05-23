data "dokploy_organization" "current" {
  name = "My Organization"
}

resource "dokploy_ssh_key" "worker" {
  organization_id = data.dokploy_organization.current.id
  name            = "worker-key"
  # private_key/public_key omitted → provider generates 4096-bit RSA.
}

output "worker_public_key" {
  value = dokploy_ssh_key.worker.public_key
  # Add this string to the remote VM's ~/.ssh/authorized_keys before creating a dokploy_server.
}
