package client

import (
	"context"
	"net/http"
	"net/url"
)

// Server is a remote machine registered as a managed worker.
type Server struct {
	ID             string `json:"serverId"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	IPAddress      string `json:"ipAddress"`
	Port           int    `json:"port"`
	Username       string `json:"username"`
	SshKeyID       string `json:"sshKeyId"`
	ServerType     string `json:"serverType"`
	OrganizationID string `json:"organizationId"`
}

// ServerInput is the create/update payload.
type ServerInput struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description"`
	IPAddress   string `json:"ipAddress,omitempty"`
	Port        int    `json:"port,omitempty"`
	Username    string `json:"username,omitempty"`
	SshKeyID    string `json:"sshKeyId,omitempty"`
	ServerType  string `json:"serverType,omitempty"`
}

func (c *Client) CreateServer(ctx context.Context, in ServerInput) (*Server, error) {
	var out Server
	if err := c.do(ctx, http.MethodPost, "server.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetServer(ctx context.Context, id string) (*Server, error) {
	var out Server
	q := url.Values{"serverId": {id}}
	if err := c.do(ctx, http.MethodGet, "server.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateServer(ctx context.Context, id string, in ServerInput) error {
	payload := struct {
		ServerInput
		ID string `json:"serverId"`
	}{ServerInput: in, ID: id}
	return c.do(ctx, http.MethodPost, "server.update", payload, nil, nil)
}

func (c *Client) DeleteServer(ctx context.Context, id string) error {
	payload := map[string]string{"serverId": id}
	return c.do(ctx, http.MethodPost, "server.remove", payload, nil, nil)
}
