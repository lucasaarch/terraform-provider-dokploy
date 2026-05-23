package provider

import (
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccDestinationResource(t *testing.T) {
	suffix := randInt()
	config := func(name string) string {
		return fmt.Sprintf(`
resource "dokploy_destination" "test" {
  name              = %q
  provider_type     = "DigitalOcean"
  bucket            = "tf-acc-bucket-%d"
  endpoint          = "https://sfo3.digitaloceanspaces.com"
  access_key        = "AKIAEXAMPLEKEY1234"
  secret_access_key = "ExampleSecret1234567890abcdef"
}`, name, suffix)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(fmt.Sprintf("tf-acc-dest-%d", suffix)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_destination.test", "id"),
					resource.TestCheckResourceAttrSet("dokploy_destination.test", "organization_id"),
					resource.TestCheckResourceAttr("dokploy_destination.test", "provider_type", "DigitalOcean"),
				),
			},
			{
				ResourceName:      "dokploy_destination.test",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				Config: config(fmt.Sprintf("tf-acc-dest-%d-renamed", suffix)),
				Check:  resource.TestCheckResourceAttr("dokploy_destination.test", "name", fmt.Sprintf("tf-acc-dest-%d-renamed", suffix)),
			},
		},
	})
}
