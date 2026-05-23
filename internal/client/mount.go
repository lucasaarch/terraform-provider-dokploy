package client

import (
	"context"
	"net/http"
	"net/url"
)

// Mount is a volume/bind/file mount on a Dokploy service.
// `serviceId` is write-only on create; the API does not return it. On read,
// the parent is identified via `serviceType` + a nullable per-type id field.
type Mount struct {
	ID          string `json:"mountId"`
	Type        string `json:"type"`
	MountPath   string `json:"mountPath"`
	HostPath    string `json:"hostPath"`
	VolumeName  string `json:"volumeName"`
	Content     string `json:"content"`
	ServiceType string `json:"serviceType"`
	// Exactly one of these will be non-null on a read response.
	ApplicationID *string `json:"applicationId"`
	ComposeID     *string `json:"composeId"`
	PostgresID    *string `json:"postgresId"`
	MysqlID       *string `json:"mysqlId"`
	MariadbID     *string `json:"mariadbId"`
	MongoID       *string `json:"mongoId"`
	RedisID       *string `json:"redisId"`
}

// ResolveServiceID returns the parent service id by inspecting the nullable
// per-type id fields populated by the API on read.
func (m *Mount) ResolveServiceID() string {
	for _, p := range []*string{m.ApplicationID, m.ComposeID, m.PostgresID, m.MysqlID, m.MariadbID, m.MongoID, m.RedisID} {
		if p != nil && *p != "" {
			return *p
		}
	}
	return ""
}

// MountInput is the create/update payload. Per-type required fields:
//
//	bind   -> HostPath
//	volume -> VolumeName
//	file   -> Content
type MountInput struct {
	ServiceID  string `json:"serviceId,omitempty"`
	Type       string `json:"type,omitempty"`
	MountPath  string `json:"mountPath,omitempty"`
	HostPath   string `json:"hostPath,omitempty"`
	VolumeName string `json:"volumeName,omitempty"`
	Content    string `json:"content,omitempty"`
}

func (c *Client) CreateMount(ctx context.Context, in MountInput) (*Mount, error) {
	var out Mount
	if err := c.do(ctx, http.MethodPost, "mounts.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetMount(ctx context.Context, id string) (*Mount, error) {
	var out Mount
	q := url.Values{"mountId": {id}}
	if err := c.do(ctx, http.MethodGet, "mounts.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateMount(ctx context.Context, id string, in MountInput) error {
	payload := struct {
		MountInput
		ID string `json:"mountId"`
	}{MountInput: in, ID: id}
	return c.do(ctx, http.MethodPost, "mounts.update", payload, nil, nil)
}

// DeleteMount calls mounts.remove (the API uses .remove, not .delete, for
// this router — verified in Task 1's API.md).
func (c *Client) DeleteMount(ctx context.Context, id string) error {
	payload := map[string]string{"mountId": id}
	return c.do(ctx, http.MethodPost, "mounts.remove", payload, nil, nil)
}
