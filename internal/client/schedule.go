package client

import (
	"context"
	"net/http"
	"net/url"
)

// Schedule is a cron-command configuration.
type Schedule struct {
	ID             string `json:"scheduleId"`
	Name           string `json:"name"`
	CronExpression string `json:"cronExpression"`
	Command        string `json:"command"`
	ShellType      string `json:"shellType"`
	ScheduleType   string `json:"scheduleType"`
	AppName        string `json:"appName"`
	ApplicationID  string `json:"applicationId"`
	ComposeID      string `json:"composeId"`
	ServerID       string `json:"serverId"`
	UserID         string `json:"userId"`
	Enabled        bool   `json:"enabled"`
	Timezone       string `json:"timezone"`
}

// ScheduleInput is the create/update payload.
type ScheduleInput struct {
	Name           string `json:"name,omitempty"`
	CronExpression string `json:"cronExpression,omitempty"`
	Command        string `json:"command,omitempty"`
	ShellType      string `json:"shellType,omitempty"`
	ScheduleType   string `json:"scheduleType,omitempty"`
	ApplicationID  string `json:"applicationId,omitempty"`
	ServerID       string `json:"serverId,omitempty"`
	Enabled        *bool  `json:"enabled,omitempty"`
	Timezone       string `json:"timezone,omitempty"`
}

func (c *Client) CreateSchedule(ctx context.Context, in ScheduleInput) (*Schedule, error) {
	var out Schedule
	if err := c.do(ctx, http.MethodPost, "schedule.create", in, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) GetSchedule(ctx context.Context, id string) (*Schedule, error) {
	var out Schedule
	q := url.Values{"scheduleId": {id}}
	if err := c.do(ctx, http.MethodGet, "schedule.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) UpdateSchedule(ctx context.Context, id string, in ScheduleInput) error {
	payload := struct {
		ScheduleInput
		ID string `json:"scheduleId"`
	}{ScheduleInput: in, ID: id}
	return c.do(ctx, http.MethodPost, "schedule.update", payload, nil, nil)
}

func (c *Client) DeleteSchedule(ctx context.Context, id string) error {
	payload := map[string]string{"scheduleId": id}
	return c.do(ctx, http.MethodPost, "schedule.delete", payload, nil, nil)
}
