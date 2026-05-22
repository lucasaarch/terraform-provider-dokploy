package client

import (
	"context"
	"net/http"
	"net/url"
)

// Project groups environments and applications. Dokploy auto-creates a
// "production" environment on project creation.
type Project struct {
	ID             string        `json:"projectId"`
	Name           string        `json:"name"`
	Description    string        `json:"description"`
	OrganizationID string        `json:"organizationId"`
	Environments   []Environment `json:"environments"`
}

// ProductionEnvironmentID returns the id of the auto-created production
// environment (the one flagged isDefault), or "" if not present.
func (p *Project) ProductionEnvironmentID() string {
	for _, e := range p.Environments {
		if e.IsDefault {
			return e.ID
		}
	}
	// Fall back to matching by name if isDefault is absent.
	for _, e := range p.Environments {
		if e.Name == "production" {
			return e.ID
		}
	}
	return ""
}

// ProjectInput is the writable payload for create/update. organizationId is
// not settable — the API key determines the organization.
type ProjectInput struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// CreateProject creates a project. The API returns a {project, environment}
// envelope (the auto-created production environment is separate); this method
// normalizes it into a Project with Environments populated, so callers can use
// ProductionEnvironmentID() uniformly.
func (c *Client) CreateProject(ctx context.Context, in ProjectInput) (*Project, error) {
	var raw struct {
		Project     Project     `json:"project"`
		Environment Environment `json:"environment"`
	}
	if err := c.do(ctx, http.MethodPost, "project.create", in, nil, &raw); err != nil {
		return nil, err
	}
	proj := raw.Project
	proj.Environments = []Environment{raw.Environment}
	return &proj, nil
}

func (c *Client) GetProject(ctx context.Context, id string) (*Project, error) {
	var out Project
	q := url.Values{"projectId": {id}}
	if err := c.do(ctx, http.MethodGet, "project.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateProject(ctx context.Context, id string, in ProjectInput) (*Project, error) {
	payload := struct {
		ProjectInput
		ID string `json:"projectId"`
	}{ProjectInput: in, ID: id}
	var out Project
	if err := c.do(ctx, http.MethodPost, "project.update", payload, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) DeleteProject(ctx context.Context, id string) error {
	payload := map[string]string{"projectId": id}
	return c.do(ctx, http.MethodPost, "project.remove", payload, nil, nil)
}
