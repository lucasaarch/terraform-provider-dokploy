package client

import (
	"context"
	"net/http"
	"net/url"
)

type Mariadb struct {
	ID                   string `json:"mariadbId"`
	Name                 string `json:"name"`
	AppName              string `json:"appName"`
	Description          string `json:"description"`
	EnvironmentID        string `json:"environmentId"`
	DockerImage          string `json:"dockerImage"`
	DatabaseName         string `json:"databaseName"`
	DatabaseUser         string `json:"databaseUser"`
	DatabasePassword     string `json:"databasePassword"`
	DatabaseRootPassword string `json:"databaseRootPassword"`
	ExternalPort         int    `json:"externalPort"`
	Env                  string `json:"env"`
	ApplicationStatus    string `json:"applicationStatus"`
}

type MariadbInput struct {
	Name                 string `json:"name"`
	AppName              string `json:"appName,omitempty"`
	Description          string `json:"description,omitempty"`
	EnvironmentID        string `json:"environmentId,omitempty"`
	DockerImage          string `json:"dockerImage,omitempty"`
	DatabaseName         string `json:"databaseName,omitempty"`
	DatabaseUser         string `json:"databaseUser,omitempty"`
	DatabasePassword     string `json:"databasePassword,omitempty"`
	DatabaseRootPassword string `json:"databaseRootPassword,omitempty"`
	ExternalPort         int    `json:"externalPort,omitempty"`
	Env                  string `json:"env,omitempty"`
}

func (c *Client) CreateMariadb(ctx context.Context, in MariadbInput) (*Mariadb, error) {
	var out Mariadb
	if err := c.do(ctx, http.MethodPost, "mariadb.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetMariadb(ctx context.Context, id string) (*Mariadb, error) {
	var out Mariadb
	q := url.Values{"mariadbId": {id}}
	if err := c.do(ctx, http.MethodGet, "mariadb.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateMariadb(ctx context.Context, id string, in MariadbInput) error {
	payload := struct {
		MariadbInput
		ID string `json:"mariadbId"`
	}{MariadbInput: in, ID: id}
	return c.do(ctx, http.MethodPost, "mariadb.update", payload, nil, nil)
}

func (c *Client) DeleteMariadb(ctx context.Context, id string) error {
	payload := map[string]string{"mariadbId": id}
	return c.do(ctx, http.MethodPost, "mariadb.remove", payload, nil, nil)
}

func (c *Client) DeployMariadb(ctx context.Context, id string) error {
	payload := map[string]string{"mariadbId": id}
	return c.do(ctx, http.MethodPost, "mariadb.deploy", payload, nil, nil)
}
