package client

import (
	"context"
	"net/http"
	"net/url"
)

type Redis struct {
	ID                string  `json:"redisId"`
	Name              string  `json:"name"`
	AppName           string  `json:"appName"`
	Description       string  `json:"description"`
	EnvironmentID     string  `json:"environmentId"`
	DockerImage       string  `json:"dockerImage"`
	DatabasePassword  string  `json:"databasePassword"`
	ExternalPort      int     `json:"externalPort"`
	Env               string  `json:"env"`
	ApplicationStatus string  `json:"applicationStatus"`
	ServerID          *string `json:"serverId"`
}

type RedisInput struct {
	Name             string  `json:"name"`
	AppName          string  `json:"appName,omitempty"`
	Description      string  `json:"description,omitempty"`
	EnvironmentID    string  `json:"environmentId,omitempty"`
	DockerImage      string  `json:"dockerImage,omitempty"`
	DatabasePassword string  `json:"databasePassword,omitempty"`
	ExternalPort     int     `json:"externalPort,omitempty"`
	Env              string  `json:"env,omitempty"`
	ServerID         *string `json:"serverId,omitempty"`
}

func (c *Client) CreateRedis(ctx context.Context, in RedisInput) (*Redis, error) {
	var out Redis
	if err := c.do(ctx, http.MethodPost, "redis.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetRedis(ctx context.Context, id string) (*Redis, error) {
	var out Redis
	q := url.Values{"redisId": {id}}
	if err := c.do(ctx, http.MethodGet, "redis.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateRedis(ctx context.Context, id string, in RedisInput) error {
	payload := struct {
		RedisInput
		ID string `json:"redisId"`
	}{RedisInput: in, ID: id}
	return c.do(ctx, http.MethodPost, "redis.update", payload, nil, nil)
}

func (c *Client) DeleteRedis(ctx context.Context, id string) error {
	payload := map[string]string{"redisId": id}
	return c.do(ctx, http.MethodPost, "redis.remove", payload, nil, nil)
}

func (c *Client) DeployRedis(ctx context.Context, id string) error {
	payload := map[string]string{"redisId": id}
	return c.do(ctx, http.MethodPost, "redis.deploy", payload, nil, nil)
}
