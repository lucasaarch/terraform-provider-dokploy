package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Application is a Docker-image-sourced application in Dokploy.
type Application struct {
	ID                string   `json:"applicationId"`
	Name              string   `json:"name"`
	Description       string   `json:"description"`
	AppName           string   `json:"appName"`
	EnvironmentID     string   `json:"environmentId"`
	DockerImage       string   `json:"dockerImage"`
	RegistryURL       string   `json:"registryUrl"`
	Username          string   `json:"username"`
	ApplicationStatus string   `json:"applicationStatus"`
	Env               string   `json:"env"`
	Backups           []Backup `json:"backups"`
}

// ApplicationInput is the application.create payload. appName is required by
// the API; Dokploy appends a random suffix to it. For application.update only
// Name/Description are sent (AppName/EnvironmentID omitted via omitempty).
type ApplicationInput struct {
	Name          string `json:"name"`
	AppName       string `json:"appName,omitempty"`
	Description   string `json:"description,omitempty"`
	EnvironmentID string `json:"environmentId,omitempty"`
}

// DockerProviderInput configures the Docker image source. The API's Zod schema
// requires registryUrl, username, and password to be present even for public
// images — username/password are pointers so they serialize as JSON null when
// unset, and registryUrl has no omitempty so it serializes as "".
type DockerProviderInput struct {
	ApplicationID string  `json:"applicationId"`
	DockerImage   string  `json:"dockerImage"`
	RegistryURL   string  `json:"registryUrl"`
	Username      *string `json:"username"`
	Password      *string `json:"password"`
}

func (c *Client) CreateApplication(ctx context.Context, in ApplicationInput) (*Application, error) {
	var out Application
	if err := c.do(ctx, http.MethodPost, "application.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetApplication(ctx context.Context, id string) (*Application, error) {
	var out Application
	q := url.Values{"applicationId": {id}}
	if err := c.do(ctx, http.MethodGet, "application.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateApplication(ctx context.Context, id string, in ApplicationInput) error {
	payload := struct {
		ApplicationInput
		ID string `json:"applicationId"`
	}{ApplicationInput: in, ID: id}
	return c.do(ctx, http.MethodPost, "application.update", payload, nil, nil)
}

func (c *Client) DeleteApplication(ctx context.Context, id string) error {
	payload := map[string]string{"applicationId": id}
	return c.do(ctx, http.MethodPost, "application.delete", payload, nil, nil)
}

// SaveDockerProvider sets the Docker image source and registry credentials.
func (c *Client) SaveDockerProvider(ctx context.Context, in DockerProviderInput) error {
	return c.do(ctx, http.MethodPost, "application.saveDockerProvider", in, nil, nil)
}

// SaveEnvironment sets the application's environment variables (dotenv string).
// The API's Zod schema requires buildArgs, buildSecrets, and createEnvFile to
// be present; buildArgs/buildSecrets are sent as null, createEnvFile as true.
func (c *Client) SaveEnvironment(ctx context.Context, applicationID, env string) error {
	payload := struct {
		ApplicationID string  `json:"applicationId"`
		Env           string  `json:"env"`
		BuildArgs     *string `json:"buildArgs"`
		BuildSecrets  *string `json:"buildSecrets"`
		CreateEnvFile bool    `json:"createEnvFile"`
	}{
		ApplicationID: applicationID,
		Env:           env,
		CreateEnvFile: true,
	}
	return c.do(ctx, http.MethodPost, "application.saveEnvironment", payload, nil, nil)
}

// Deploy triggers an asynchronous deployment.
func (c *Client) Deploy(ctx context.Context, applicationID string) error {
	payload := map[string]string{"applicationId": applicationID}
	return c.do(ctx, http.MethodPost, "application.deploy", payload, nil, nil)
}

// WaitForDeployment polls application status until it reaches "done" or
// "error". It returns an error on "error" status or when ctx is cancelled.
func (c *Client) WaitForDeployment(ctx context.Context, applicationID string, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		app, err := c.GetApplication(ctx, applicationID)
		if err != nil {
			return err
		}
		switch app.ApplicationStatus {
		case "done":
			return nil
		case "error":
			return fmt.Errorf("deployment failed (applicationStatus=error); check deploy logs in the Dokploy dashboard for application %q", applicationID)
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("timed out or cancelled waiting for deployment of %q: %w", applicationID, ctx.Err())
		case <-ticker.C:
		}
	}
}
