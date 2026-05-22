package client

import (
	"context"
	"net/http"
	"net/url"
)

// Environment is a deployment environment inside a project.
type Environment struct {
	ID          string `json:"environmentId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	ProjectID   string `json:"projectId"`
	// IsDefault is true for the auto-created production environment.
	IsDefault bool `json:"isDefault"`
	// Env holds shared variables in dotenv format ("KEY=value\nKEY2=value2").
	Env string `json:"env"`
}

// EnvironmentInput is the writable payload for create/update.
// All fields use omitempty so partial updates (e.g. env-only) do not send
// empty strings that fail the server's Zod min-length validation.
type EnvironmentInput struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	ProjectID   string `json:"projectId,omitempty"`
	Env         string `json:"env,omitempty"`
}

func (c *Client) CreateEnvironment(ctx context.Context, in EnvironmentInput) (*Environment, error) {
	var out Environment
	if err := c.do(ctx, http.MethodPost, "environment.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetEnvironment(ctx context.Context, id string) (*Environment, error) {
	var out Environment
	q := url.Values{"environmentId": {id}}
	if err := c.do(ctx, http.MethodGet, "environment.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateEnvironment(ctx context.Context, id string, in EnvironmentInput) (*Environment, error) {
	payload := struct {
		EnvironmentInput
		ID string `json:"environmentId"`
	}{EnvironmentInput: in, ID: id}
	var out Environment
	if err := c.do(ctx, http.MethodPost, "environment.update", payload, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteEnvironment(ctx context.Context, id string) error {
	payload := map[string]string{"environmentId": id}
	return c.do(ctx, http.MethodPost, "environment.remove", payload, nil, nil)
}
