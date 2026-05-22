# The organization the configured API key belongs to.
data "dokploy_organization" "current" {}

output "organization_name" {
  value = data.dokploy_organization.current.name
}
