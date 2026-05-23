package client

import (
	"context"
	"net/http"
	"net/url"
)

type Mongo struct {
	ID                string `json:"mongoId"`
	Name              string `json:"name"`
	AppName           string `json:"appName"`
	Description       string `json:"description"`
	EnvironmentID     string `json:"environmentId"`
	DockerImage       string `json:"dockerImage"`
	DatabaseUser      string `json:"databaseUser"`
	DatabasePassword  string `json:"databasePassword"`
	ExternalPort      int    `json:"externalPort"`
	Env               string `json:"env"`
	ApplicationStatus string `json:"applicationStatus"`
}

type MongoInput struct {
	Name             string `json:"name"`
	AppName          string `json:"appName,omitempty"`
	Description      string `json:"description,omitempty"`
	EnvironmentID    string `json:"environmentId,omitempty"`
	DockerImage      string `json:"dockerImage,omitempty"`
	DatabaseUser     string `json:"databaseUser,omitempty"`
	DatabasePassword string `json:"databasePassword,omitempty"`
	ExternalPort     int    `json:"externalPort,omitempty"`
	Env              string `json:"env,omitempty"`
}

func (c *Client) CreateMongo(ctx context.Context, in MongoInput) (*Mongo, error) {
	var out Mongo
	if err := c.do(ctx, http.MethodPost, "mongo.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetMongo(ctx context.Context, id string) (*Mongo, error) {
	var out Mongo
	q := url.Values{"mongoId": {id}}
	if err := c.do(ctx, http.MethodGet, "mongo.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateMongo(ctx context.Context, id string, in MongoInput) error {
	payload := struct {
		MongoInput
		ID string `json:"mongoId"`
	}{MongoInput: in, ID: id}
	return c.do(ctx, http.MethodPost, "mongo.update", payload, nil, nil)
}

func (c *Client) DeleteMongo(ctx context.Context, id string) error {
	payload := map[string]string{"mongoId": id}
	return c.do(ctx, http.MethodPost, "mongo.remove", payload, nil, nil)
}

func (c *Client) DeployMongo(ctx context.Context, id string) error {
	payload := map[string]string{"mongoId": id}
	return c.do(ctx, http.MethodPost, "mongo.deploy", payload, nil, nil)
}
