package client

import (
	"context"
	"net/http"
	"net/url"
)

// Postgres is a Dokploy-managed PostgreSQL service.
type Postgres struct {
	ID                string `json:"postgresId"`
	Name              string `json:"name"`
	AppName           string `json:"appName"`
	Description       string `json:"description"`
	EnvironmentID     string `json:"environmentId"`
	DockerImage       string `json:"dockerImage"`
	DatabaseName      string `json:"databaseName"`
	DatabaseUser      string `json:"databaseUser"`
	DatabasePassword  string `json:"databasePassword"`
	ExternalPort      int    `json:"externalPort"`
	Env               string `json:"env"`
	ApplicationStatus string `json:"applicationStatus"`
}

// PostgresInput is the create/update payload.
type PostgresInput struct {
	Name             string `json:"name"`
	AppName          string `json:"appName,omitempty"`
	Description      string `json:"description,omitempty"`
	EnvironmentID    string `json:"environmentId,omitempty"`
	DockerImage      string `json:"dockerImage,omitempty"`
	DatabaseName     string `json:"databaseName,omitempty"`
	DatabaseUser     string `json:"databaseUser,omitempty"`
	DatabasePassword string `json:"databasePassword,omitempty"`
	ExternalPort     int    `json:"externalPort,omitempty"`
	Env              string `json:"env,omitempty"`
}

func (c *Client) CreatePostgres(ctx context.Context, in PostgresInput) (*Postgres, error) {
	var out Postgres
	if err := c.do(ctx, http.MethodPost, "postgres.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetPostgres(ctx context.Context, id string) (*Postgres, error) {
	var out Postgres
	q := url.Values{"postgresId": {id}}
	if err := c.do(ctx, http.MethodGet, "postgres.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdatePostgres(ctx context.Context, id string, in PostgresInput) error {
	payload := struct {
		PostgresInput
		ID string `json:"postgresId"`
	}{PostgresInput: in, ID: id}
	return c.do(ctx, http.MethodPost, "postgres.update", payload, nil, nil)
}

func (c *Client) DeletePostgres(ctx context.Context, id string) error {
	payload := map[string]string{"postgresId": id}
	return c.do(ctx, http.MethodPost, "postgres.remove", payload, nil, nil)
}

// DeployPostgres triggers an asynchronous deployment of an existing service.
func (c *Client) DeployPostgres(ctx context.Context, id string) error {
	payload := map[string]string{"postgresId": id}
	return c.do(ctx, http.MethodPost, "postgres.deploy", payload, nil, nil)
}
