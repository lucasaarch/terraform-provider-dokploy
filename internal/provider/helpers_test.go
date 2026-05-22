package provider

import (
	"context"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/lucasaarch/terraform-provider-dokploy/internal/client"
)

var rng = rand.New(rand.NewSource(time.Now().UnixNano()))

// randInt returns a positive random int for unique acceptance-test names.
func randInt() int {
	return rng.Intn(1_000_000)
}

// firstOrgName returns the name of the first organization the API key can see.
// Used to feed acceptance tests a valid organization name without hardcoding
// an instance-specific value.
func firstOrgName(t *testing.T) string {
	t.Helper()
	c := client.New(os.Getenv("DOKPLOY_ENDPOINT"), os.Getenv("DOKPLOY_API_KEY"))
	orgs, err := c.ListOrganizations(context.Background())
	if err != nil {
		t.Fatalf("firstOrgName: ListOrganizations failed: %v", err)
	}
	if len(orgs) == 0 {
		t.Fatal("firstOrgName: no organizations visible to the API key")
	}
	return orgs[0].Name
}
