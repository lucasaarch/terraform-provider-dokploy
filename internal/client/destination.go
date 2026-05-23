package client

import (
	"context"
	"net/http"
	"net/url"
)

// Destination is an S3-compatible storage configuration at the organization level.
type Destination struct {
	ID              string   `json:"destinationId"`
	Name            string   `json:"name"`
	Provider        string   `json:"provider"`
	Bucket          string   `json:"bucket"`
	Endpoint        string   `json:"endpoint"`
	Region          string   `json:"region"`
	AccessKey       string   `json:"accessKey"`
	SecretAccessKey string   `json:"secretAccessKey"`
	AdditionalFlags []string `json:"additionalFlags"`
	OrganizationID  string   `json:"organizationId"`
}

// DestinationInput is the create/update payload. Region is intentionally
// without omitempty: the Dokploy API's Zod schema rejects requests with a
// missing region field even when it would be empty.
type DestinationInput struct {
	Name            string   `json:"name,omitempty"`
	Provider        string   `json:"provider,omitempty"`
	Bucket          string   `json:"bucket,omitempty"`
	Endpoint        string   `json:"endpoint,omitempty"`
	Region          string   `json:"region"`
	AccessKey       string   `json:"accessKey,omitempty"`
	SecretAccessKey string   `json:"secretAccessKey,omitempty"`
	AdditionalFlags []string `json:"additionalFlags,omitempty"`
}

func (c *Client) CreateDestination(ctx context.Context, in DestinationInput) (*Destination, error) {
	var out Destination
	if err := c.do(ctx, http.MethodPost, "destination.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetDestination(ctx context.Context, id string) (*Destination, error) {
	var out Destination
	q := url.Values{"destinationId": {id}}
	if err := c.do(ctx, http.MethodGet, "destination.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateDestination(ctx context.Context, id string, in DestinationInput) error {
	payload := struct {
		DestinationInput
		ID string `json:"destinationId"`
	}{DestinationInput: in, ID: id}
	return c.do(ctx, http.MethodPost, "destination.update", payload, nil, nil)
}

func (c *Client) DeleteDestination(ctx context.Context, id string) error {
	payload := map[string]string{"destinationId": id}
	return c.do(ctx, http.MethodPost, "destination.remove", payload, nil, nil)
}
