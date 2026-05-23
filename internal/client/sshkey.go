package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// SshKey is an SSH key registered in Dokploy.
type SshKey struct {
	ID             string `json:"sshKeyId"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	PublicKey      string `json:"publicKey"`
	PrivateKey     string `json:"privateKey"`
	OrganizationID string `json:"organizationId"`
}

// SshKeyInput is the create payload (name + keys + org + optional description).
type SshKeyInput struct {
	Name           string `json:"name,omitempty"`
	Description    string `json:"description,omitempty"`
	PublicKey      string `json:"publicKey,omitempty"`
	PrivateKey     string `json:"privateKey,omitempty"`
	OrganizationID string `json:"organizationId,omitempty"`
}

// SshKeyUpdateInput is the restricted update payload — only name/description.
type SshKeyUpdateInput struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// ListSshKeys returns every SSH key visible to the API key.
func (c *Client) ListSshKeys(ctx context.Context) ([]SshKey, error) {
	var out []SshKey
	if err := c.do(ctx, http.MethodGet, "sshKey.all", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// CreateSshKey creates an SSH key. The API responds with an empty body, so the
// new key's id is discovered by diffing sshKey.all before and after.
func (c *Client) CreateSshKey(ctx context.Context, in SshKeyInput) (*SshKey, error) {
	before, err := c.ListSshKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing ssh keys before create: %w", err)
	}
	seen := make(map[string]struct{}, len(before))
	for _, k := range before {
		seen[k.ID] = struct{}{}
	}

	if err := c.do(ctx, http.MethodPost, "sshKey.create", in, nil, nil); err != nil {
		return nil, err
	}

	after, err := c.ListSshKeys(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing ssh keys after create: %w", err)
	}
	for i := range after {
		if _, was := seen[after[i].ID]; !was {
			return &after[i], nil
		}
	}
	return nil, fmt.Errorf("sshKey.create returned 200 but no new key found")
}

func (c *Client) GetSshKey(ctx context.Context, id string) (*SshKey, error) {
	var out SshKey
	q := url.Values{"sshKeyId": {id}}
	if err := c.do(ctx, http.MethodGet, "sshKey.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateSshKey updates name and/or description. The keys themselves are
// immutable — changing them requires destroying and recreating the resource.
func (c *Client) UpdateSshKey(ctx context.Context, id string, in SshKeyUpdateInput) error {
	payload := struct {
		SshKeyUpdateInput
		ID string `json:"sshKeyId"`
	}{SshKeyUpdateInput: in, ID: id}
	return c.do(ctx, http.MethodPost, "sshKey.update", payload, nil, nil)
}

func (c *Client) DeleteSshKey(ctx context.Context, id string) error {
	payload := map[string]string{"sshKeyId": id}
	return c.do(ctx, http.MethodPost, "sshKey.remove", payload, nil, nil)
}
