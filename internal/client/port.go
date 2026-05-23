package client

import (
	"context"
	"net/http"
	"net/url"
)

// Port is a published port mapping on an application.
type Port struct {
	ID            string `json:"portId"`
	ApplicationID string `json:"applicationId"`
	PublishedPort int    `json:"publishedPort"`
	TargetPort    int    `json:"targetPort"`
	Protocol      string `json:"protocol"`
}

// PortInput is the create/update payload.
type PortInput struct {
	ApplicationID string `json:"applicationId,omitempty"`
	PublishedPort int    `json:"publishedPort,omitempty"`
	TargetPort    int    `json:"targetPort,omitempty"`
	Protocol      string `json:"protocol,omitempty"`
}

func (c *Client) CreatePort(ctx context.Context, in PortInput) (*Port, error) {
	var out Port
	if err := c.do(ctx, http.MethodPost, "port.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetPort(ctx context.Context, id string) (*Port, error) {
	var out Port
	q := url.Values{"portId": {id}}
	if err := c.do(ctx, http.MethodGet, "port.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdatePort(ctx context.Context, id string, in PortInput) error {
	payload := struct {
		PortInput
		ID string `json:"portId"`
	}{PortInput: in, ID: id}
	return c.do(ctx, http.MethodPost, "port.update", payload, nil, nil)
}

// DeletePort calls port.delete (the API uses .delete, not .remove, for
// this router — verified in Task 1's API.md).
func (c *Client) DeletePort(ctx context.Context, id string) error {
	payload := map[string]string{"portId": id}
	return c.do(ctx, http.MethodPost, "port.delete", payload, nil, nil)
}
