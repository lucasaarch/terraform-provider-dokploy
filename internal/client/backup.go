package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// Backup is a scheduled backup configuration.
type Backup struct {
	ID              string          `json:"backupId"`
	AppName         string          `json:"appName"`
	Schedule        string          `json:"schedule"`
	Prefix          string          `json:"prefix"`
	DestinationID   string          `json:"destinationId"`
	Database        string          `json:"database"`
	DatabaseType    string          `json:"databaseType"`
	BackupType      string          `json:"backupType"`
	Enabled         *bool           `json:"enabled"`
	KeepLatestCount *int            `json:"keepLatestCount"`
	ServiceName     *string         `json:"serviceName"`
	Metadata        json.RawMessage `json:"metadata"`
	PostgresID      *string         `json:"postgresId"`
	MysqlID         *string         `json:"mysqlId"`
	MariadbID       *string         `json:"mariadbId"`
	MongoID         *string         `json:"mongoId"`
	LibsqlID        *string         `json:"libsqlId"`
	ComposeID       *string         `json:"composeId"`
}

// BackupInput is the create/update payload. The typed id (PostgresID, etc.) is
// required on create alongside Database; the helper SetTypedID derives it.
// On update, all nullable fields must be present in the JSON — pointer fields
// with no `omitempty` serialize as JSON null when nil.
type BackupInput struct {
	Schedule        string          `json:"schedule"`
	Prefix          string          `json:"prefix"`
	DestinationID   string          `json:"destinationId"`
	Database        string          `json:"database"`
	DatabaseType    string          `json:"databaseType"`
	Enabled         *bool           `json:"enabled"`
	KeepLatestCount *int            `json:"keepLatestCount"`
	ServiceName     *string         `json:"serviceName"`
	Metadata        json.RawMessage `json:"metadata"`
	PostgresID      *string         `json:"postgresId,omitempty"`
	MysqlID         *string         `json:"mysqlId,omitempty"`
	MariadbID       *string         `json:"mariadbId,omitempty"`
	MongoID         *string         `json:"mongoId,omitempty"`
	LibsqlID        *string         `json:"libsqlId,omitempty"`
}

// SetTypedID populates the typed id field (PostgresID, MysqlID, …) based on
// DatabaseType. Returns an error for an unknown type.
func (in *BackupInput) SetTypedID() error {
	id := in.Database
	switch in.DatabaseType {
	case "postgres":
		in.PostgresID = &id
	case "mysql":
		in.MysqlID = &id
	case "mariadb":
		in.MariadbID = &id
	case "mongo":
		in.MongoID = &id
	case "libsql":
		in.LibsqlID = &id
	case "web-server":
		// web-server has no typed id field; the API accepts `database` alone for this type.
	default:
		return fmt.Errorf("unknown databaseType %q", in.DatabaseType)
	}
	return nil
}

// CreateBackup creates a backup. The API responds with an empty body, so the
// new backup id is discovered by diffing the parent resource's backups[] list.
func (c *Client) CreateBackup(ctx context.Context, in BackupInput) (*Backup, error) {
	if err := in.SetTypedID(); err != nil {
		return nil, err
	}

	before, err := c.listBackupsForResource(ctx, in.DatabaseType, in.Database)
	if err != nil {
		return nil, fmt.Errorf("listing backups before create: %w", err)
	}
	seen := make(map[string]struct{}, len(before))
	for _, b := range before {
		seen[b.ID] = struct{}{}
	}

	if err := c.do(ctx, http.MethodPost, "backup.create", in, nil, nil); err != nil {
		return nil, err
	}

	after, err := c.listBackupsForResource(ctx, in.DatabaseType, in.Database)
	if err != nil {
		return nil, fmt.Errorf("listing backups after create: %w", err)
	}
	for i := range after {
		if _, was := seen[after[i].ID]; !was {
			return &after[i], nil
		}
	}
	return nil, fmt.Errorf("backup.create returned 200 but no new backup found on the parent %s", in.DatabaseType)
}

// listBackupsForResource fetches the parent resource and returns its backups[].
func (c *Client) listBackupsForResource(ctx context.Context, dbType, id string) ([]Backup, error) {
	switch dbType {
	case "postgres":
		pg, err := c.GetPostgres(ctx, id)
		if err != nil {
			return nil, err
		}
		return pg.Backups, nil
	case "mysql":
		my, err := c.GetMysql(ctx, id)
		if err != nil {
			return nil, err
		}
		return my.Backups, nil
	case "mariadb":
		ma, err := c.GetMariadb(ctx, id)
		if err != nil {
			return nil, err
		}
		return ma.Backups, nil
	case "mongo":
		mo, err := c.GetMongo(ctx, id)
		if err != nil {
			return nil, err
		}
		return mo.Backups, nil
	case "web-server":
		return nil, fmt.Errorf("listing web-server backups is not yet supported: application.one does not return backups[] — web-server database_type cannot be used with this provider version")
	default:
		return nil, fmt.Errorf("unknown databaseType %q", dbType)
	}
}

func (c *Client) GetBackup(ctx context.Context, id string) (*Backup, error) {
	var out Backup
	q := url.Values{"backupId": {id}}
	if err := c.do(ctx, http.MethodGet, "backup.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateBackup sends every writable field (the API requires a full payload,
// not a partial update).
func (c *Client) UpdateBackup(ctx context.Context, id string, in BackupInput) error {
	payload := struct {
		BackupInput
		ID string `json:"backupId"`
	}{BackupInput: in, ID: id}
	return c.do(ctx, http.MethodPost, "backup.update", payload, nil, nil)
}

func (c *Client) DeleteBackup(ctx context.Context, id string) error {
	payload := map[string]string{"backupId": id}
	return c.do(ctx, http.MethodPost, "backup.remove", payload, nil, nil)
}
