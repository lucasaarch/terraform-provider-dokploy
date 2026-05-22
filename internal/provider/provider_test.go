package provider

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testAccProtoV6ProviderFactories is used by every acceptance test.
var testAccProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"dokploy": providerserver.NewProtocol6WithError(New("test")()),
}

// testAccPreCheck verifies required env vars are set before acceptance tests.
func testAccPreCheck(t *testing.T) {
	t.Helper()
	for _, k := range []string{"DOKPLOY_ENDPOINT", "DOKPLOY_API_KEY"} {
		if v := getEnv(k); v == "" {
			t.Fatalf("%s must be set for acceptance tests", k)
		}
	}
}

func TestProvider_Schema(t *testing.T) {
	p := New("test")()
	if p == nil {
		t.Fatal("New() returned nil provider")
	}
}
