package client

import (
	"context"
	"net/http"
	"net/url"
)

// Compose is a Docker Compose stack managed by Dokploy.
// The deploy-status field is named composeStatus (NOT applicationStatus).
type Compose struct {
	ID            string   `json:"composeId"`
	Name          string   `json:"name"`
	AppName       string   `json:"appName"`
	Description   string   `json:"description"`
	EnvironmentID string   `json:"environmentId"`
	ServerID      *string  `json:"serverId"`
	SourceType    string   `json:"sourceType"`
	ComposeFile   string   `json:"composeFile"`
	Env           string   `json:"env"`
	ComposeStatus string   `json:"composeStatus"`
	Backups       []Backup `json:"backups"`
}

// ComposeInput is the create/update payload.
type ComposeInput struct {
	Name          string  `json:"name,omitempty"`
	AppName       string  `json:"appName,omitempty"`
	Description   string  `json:"description,omitempty"`
	EnvironmentID string  `json:"environmentId,omitempty"`
	ServerID      *string `json:"serverId,omitempty"`
	SourceType    string  `json:"sourceType,omitempty"`
	ComposeFile   string  `json:"composeFile,omitempty"`
	Env           string  `json:"env,omitempty"`
}

func (c *Client) CreateCompose(ctx context.Context, in ComposeInput) (*Compose, error) {
	var out Compose
	if err := c.do(ctx, http.MethodPost, "compose.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetCompose(ctx context.Context, id string) (*Compose, error) {
	var out Compose
	q := url.Values{"composeId": {id}}
	if err := c.do(ctx, http.MethodGet, "compose.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateCompose(ctx context.Context, id string, in ComposeInput) error {
	payload := struct {
		ComposeInput
		ID string `json:"composeId"`
	}{ComposeInput: in, ID: id}
	return c.do(ctx, http.MethodPost, "compose.update", payload, nil, nil)
}

// DeleteCompose calls compose.delete (the API uses .delete, not .remove, for
// this router — verified in Task 1's API.md).
func (c *Client) DeleteCompose(ctx context.Context, id string) error {
	payload := map[string]string{"composeId": id}
	return c.do(ctx, http.MethodPost, "compose.delete", payload, nil, nil)
}

// DeployCompose triggers an asynchronous deployment of the stack.
func (c *Client) DeployCompose(ctx context.Context, id string) error {
	payload := map[string]string{"composeId": id}
	return c.do(ctx, http.MethodPost, "compose.deploy", payload, nil, nil)
}
