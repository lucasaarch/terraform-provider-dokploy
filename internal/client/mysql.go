package client

import (
	"context"
	"net/http"
	"net/url"
)

// Mysql is a Dokploy-managed MySQL service.
type Mysql struct {
	ID                   string `json:"mysqlId"`
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

// MysqlInput is the create/update payload.
type MysqlInput struct {
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

func (c *Client) CreateMysql(ctx context.Context, in MysqlInput) (*Mysql, error) {
	var out Mysql
	if err := c.do(ctx, http.MethodPost, "mysql.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetMysql(ctx context.Context, id string) (*Mysql, error) {
	var out Mysql
	q := url.Values{"mysqlId": {id}}
	if err := c.do(ctx, http.MethodGet, "mysql.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateMysql(ctx context.Context, id string, in MysqlInput) error {
	payload := struct {
		MysqlInput
		ID string `json:"mysqlId"`
	}{MysqlInput: in, ID: id}
	return c.do(ctx, http.MethodPost, "mysql.update", payload, nil, nil)
}

func (c *Client) DeleteMysql(ctx context.Context, id string) error {
	payload := map[string]string{"mysqlId": id}
	return c.do(ctx, http.MethodPost, "mysql.remove", payload, nil, nil)
}

func (c *Client) DeployMysql(ctx context.Context, id string) error {
	payload := map[string]string{"mysqlId": id}
	return c.do(ctx, http.MethodPost, "mysql.deploy", payload, nil, nil)
}
