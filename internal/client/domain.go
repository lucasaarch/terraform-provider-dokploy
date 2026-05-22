package client

import (
	"context"
	"net/http"
	"net/url"
)

// Domain routes external traffic to an application.
type Domain struct {
	ID              string `json:"domainId"`
	Host            string `json:"host"`
	Path            string `json:"path"`
	Port            int    `json:"port"`
	HTTPS           bool   `json:"https"`
	CertificateType string `json:"certificateType"`
	ApplicationID   string `json:"applicationId"`
}

// DomainInput is the writable payload for create/update. The API's Zod schema
// requires host, port, https, path, and certificateType on create — none use
// omitempty so they always serialize.
type DomainInput struct {
	Host            string `json:"host"`
	Path            string `json:"path"`
	Port            int    `json:"port"`
	HTTPS           bool   `json:"https"`
	CertificateType string `json:"certificateType"`
	ApplicationID   string `json:"applicationId,omitempty"`
}

func (c *Client) CreateDomain(ctx context.Context, in DomainInput) (*Domain, error) {
	var out Domain
	if err := c.do(ctx, http.MethodPost, "domain.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetDomain(ctx context.Context, id string) (*Domain, error) {
	var out Domain
	q := url.Values{"domainId": {id}}
	if err := c.do(ctx, http.MethodGet, "domain.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateDomain(ctx context.Context, id string, in DomainInput) (*Domain, error) {
	payload := struct {
		DomainInput
		ID string `json:"domainId"`
	}{DomainInput: in, ID: id}
	var out Domain
	if err := c.do(ctx, http.MethodPost, "domain.update", payload, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteDomain(ctx context.Context, id string) error {
	payload := map[string]string{"domainId": id}
	return c.do(ctx, http.MethodPost, "domain.delete", payload, nil, nil)
}
