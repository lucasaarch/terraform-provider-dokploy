terraform {
  required_providers {
    dokploy = {
      source = "lucasaarch/dokploy"
    }
  }
}

provider "dokploy" {
  endpoint = "https://dokploy.example.com"
  # api_key is read from the DOKPLOY_API_KEY environment variable.
}
