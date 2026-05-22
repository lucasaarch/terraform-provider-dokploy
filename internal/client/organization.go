package client

import (
	"context"
	"net/http"
)

// Organization is the tenancy object an API key is bound to. Read-only:
// organizations cannot be created or modified through the Dokploy API.
type Organization struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// ListOrganizations returns every organization visible to the API key. In
// practice an API key sees exactly one organization.
func (c *Client) ListOrganizations(ctx context.Context) ([]Organization, error) {
	var out []Organization
	if err := c.do(ctx, http.MethodGet, "organization.all", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}
