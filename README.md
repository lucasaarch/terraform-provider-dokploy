# Terraform Provider for Dokploy

Manage [Dokploy](https://dokploy.com) infrastructure declaratively with Terraform.

## Resources

- `dokploy_project` — project plus its auto-created `production` environment
- `dokploy_environment` — custom environments
- `dokploy_application` — Docker-image applications (deploys on apply)
- `dokploy_domain` — domains routing traffic to applications

## Data sources

- `dokploy_organization` — the organization the API key belongs to (read-only)

## Provider configuration

```hcl
provider "dokploy" {
  endpoint = "https://dokploy.example.com" # or DOKPLOY_ENDPOINT
  # api_key via DOKPLOY_API_KEY
}
```

## Development

- `make build` — build the provider binary
- `make test` — run unit tests (no network)
- `make testacc` — run acceptance tests (needs `DOKPLOY_ENDPOINT` and `DOKPLOY_API_KEY`)
- `make docs` — regenerate documentation

## License

MPL-2.0
