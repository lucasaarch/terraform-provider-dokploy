package provider

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccSshKeyResource(t *testing.T) {
	suffix := randInt()
	config := func(name string) string {
		return fmt.Sprintf(`
data "dokploy_organization" "current" {
  name = %q
}

resource "dokploy_ssh_key" "test" {
  organization_id = data.dokploy_organization.current.id
  name            = %q
  # private_key/public_key omitted → provider generates 4096-bit RSA.
}`, firstOrgName(t), name)
	}

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: config(fmt.Sprintf("tf-acc-sshkey-%d", suffix)),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttrSet("dokploy_ssh_key.test", "id"),
					resource.TestMatchResourceAttr("dokploy_ssh_key.test", "public_key",
						regexp.MustCompile(`^ssh-rsa AAAA[A-Za-z0-9+/=]+`)),
					resource.TestMatchResourceAttr("dokploy_ssh_key.test", "private_key",
						regexp.MustCompile(`(?s)^-----BEGIN RSA PRIVATE KEY-----.*-----END RSA PRIVATE KEY-----`)),
				),
			},
			{
				ResourceName:      "dokploy_ssh_key.test",
				ImportState:       true,
				ImportStateVerify: true,
				// private_key: API may or may not return it in plaintext.
				// organization_id: API returns a different org id than was sent on create
				// (the key gets associated with the user's personal/default org internally).
				ImportStateVerifyIgnore: []string{"private_key", "organization_id"},
			},
			{
				Config: config(fmt.Sprintf("tf-acc-sshkey-%d-renamed", suffix)),
				Check:  resource.TestCheckResourceAttr("dokploy_ssh_key.test", "name", fmt.Sprintf("tf-acc-sshkey-%d-renamed", suffix)),
			},
		},
	})
}
