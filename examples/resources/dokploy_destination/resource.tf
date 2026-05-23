resource "dokploy_destination" "s3" {
  name              = "prod-backups"
  provider_type     = "DigitalOcean"
  bucket            = "my-bucket"
  endpoint          = "https://sfo3.digitaloceanspaces.com"
  region            = ""
  access_key        = var.do_access_key
  secret_access_key = var.do_secret_key
}
