package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

// EventFlags are the eight event toggles common to every notification type.
type EventFlags struct {
	AppDeploy       bool `json:"appDeploy"`
	AppBuildError   bool `json:"appBuildError"`
	DatabaseBackup  bool `json:"databaseBackup"`
	DokployBackup   bool `json:"dokployBackup"`
	VolumeBackup    bool `json:"volumeBackup"`
	DokployRestart  bool `json:"dokployRestart"`
	DockerCleanup   bool `json:"dockerCleanup"`
	ServerThreshold bool `json:"serverThreshold"`
}

// NotificationTypeDetail holds the type-specific sub-object embedded in the
// notification.all / notification.one responses. Each field is nil when not
// applicable.
type NotificationTypeDetail struct {
	SlackID    *string `json:"slackId"`
	DiscordID  *string `json:"discordId"`
	EmailID    *string `json:"emailId"`
	TelegramID *string `json:"telegramId"`
	GotifyID   *string `json:"gotifyId"`
}

// NotificationSlack is the embedded slack sub-object in notification.all.
type NotificationSlack struct {
	SlackID    string `json:"slackId"`
	WebhookURL string `json:"webhookUrl"`
	Channel    string `json:"channel"`
}

// NotificationDiscord is the embedded discord sub-object.
type NotificationDiscord struct {
	DiscordID  string `json:"discordId"`
	WebhookURL string `json:"webhookUrl"`
	Decoration bool   `json:"decoration"`
}

// NotificationEmail is the embedded email sub-object.
type NotificationEmail struct {
	EmailID     string   `json:"emailId"`
	SMTPServer  string   `json:"smtpServer"`
	SMTPPort    int      `json:"smtpPort"`
	Username    string   `json:"username"`
	Password    string   `json:"password"`
	FromAddress string   `json:"fromAddress"`
	ToAddresses []string `json:"toAddresses"`
}

// NotificationTelegram is the embedded telegram sub-object.
type NotificationTelegram struct {
	TelegramID      string `json:"telegramId"`
	BotToken        string `json:"botToken"`
	ChatID          string `json:"chatId"`
	MessageThreadID string `json:"messageThreadId"`
}

// NotificationGotify is the embedded gotify sub-object.
type NotificationGotify struct {
	GotifyID   string `json:"gotifyId"`
	ServerURL  string `json:"serverUrl"`
	AppToken   string `json:"appToken"`
	Priority   int    `json:"priority"`
	Decoration bool   `json:"decoration"`
}

// Notification is the read shape returned by notification.one and notification.all.
// Type-specific sub-objects are embedded; the presence varies based on NotificationType.
type Notification struct {
	ID               string `json:"notificationId"`
	Name             string `json:"name"`
	NotificationType string `json:"notificationType"`
	CreatedAt        string `json:"createdAt"`
	OrganizationID   string `json:"organizationId"`
	EventFlags
	// Type-specific sub-ids (from the top-level notification object)
	SlackID    *string `json:"slackId"`
	DiscordID  *string `json:"discordId"`
	EmailID    *string `json:"emailId"`
	TelegramID *string `json:"telegramId"`
	GotifyID   *string `json:"gotifyId"`
	// Type-specific sub-objects (embedded for credential access)
	Slack    *NotificationSlack    `json:"slack"`
	Discord  *NotificationDiscord  `json:"discord"`
	Email    *NotificationEmail    `json:"email"`
	Telegram *NotificationTelegram `json:"telegram"`
	Gotify   *NotificationGotify   `json:"gotify"`
}

// SlackNotificationInput is the payload for notification.createSlack / updateSlack.
type SlackNotificationInput struct {
	Name       string `json:"name,omitempty"`
	WebhookURL string `json:"webhookUrl,omitempty"`
	Channel    string `json:"channel,omitempty"`
	EventFlags
}

// DiscordNotificationInput is the payload for notification.createDiscord / updateDiscord.
type DiscordNotificationInput struct {
	Name       string `json:"name,omitempty"`
	WebhookURL string `json:"webhookUrl,omitempty"`
	Decoration *bool  `json:"decoration,omitempty"`
	EventFlags
}

// EmailNotificationInput is the payload for notification.createEmail / updateEmail.
type EmailNotificationInput struct {
	Name        string   `json:"name,omitempty"`
	SMTPServer  string   `json:"smtpServer,omitempty"`
	SMTPPort    int      `json:"smtpPort,omitempty"`
	Username    string   `json:"username,omitempty"`
	Password    string   `json:"password,omitempty"`
	FromAddress string   `json:"fromAddress,omitempty"`
	ToAddresses []string `json:"toAddresses,omitempty"`
	EventFlags
}

// TelegramNotificationInput is the payload for notification.createTelegram / updateTelegram.
type TelegramNotificationInput struct {
	Name            string `json:"name,omitempty"`
	BotToken        string `json:"botToken,omitempty"`
	ChatID          string `json:"chatId,omitempty"`
	MessageThreadID string `json:"messageThreadId"` // required by API even as ""
	EventFlags
}

// GotifyNotificationInput is the payload for notification.createGotify / updateGotify.
type GotifyNotificationInput struct {
	Name       string `json:"name,omitempty"`
	ServerURL  string `json:"serverUrl,omitempty"`
	AppToken   string `json:"appToken,omitempty"`
	Priority   int    `json:"priority"`   // required by API as a number
	Decoration bool   `json:"decoration"` // required by API as a boolean
	EventFlags
}

// ListNotifications returns all notifications in the organization.
// Used internally by Create* methods to discover new notification IDs.
func (c *Client) ListNotifications(ctx context.Context) ([]Notification, error) {
	var out []Notification
	if err := c.do(ctx, http.MethodGet, "notification.all", nil, nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// createNotificationAndFindID wraps the empty-body create pattern:
// list before, call createFn, list after, return the new notification.
func (c *Client) createNotificationAndFindID(ctx context.Context, createFn func() error) (*Notification, error) {
	before, err := c.ListNotifications(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing notifications before create: %w", err)
	}
	seen := make(map[string]struct{}, len(before))
	for _, n := range before {
		seen[n.ID] = struct{}{}
	}

	if err := createFn(); err != nil {
		return nil, err
	}

	after, err := c.ListNotifications(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing notifications after create: %w", err)
	}
	for i := range after {
		if _, was := seen[after[i].ID]; !was {
			return &after[i], nil
		}
	}
	return nil, fmt.Errorf("notification create returned 200 but no new notification found")
}

// CreateSlackNotification creates a Slack notification.
// The API returns empty body; this method discovers the new ID via diff of notification.all.
func (c *Client) CreateSlackNotification(ctx context.Context, in SlackNotificationInput) (*Notification, error) {
	return c.createNotificationAndFindID(ctx, func() error {
		return c.do(ctx, http.MethodPost, "notification.createSlack", in, nil, nil)
	})
}

// CreateDiscordNotification creates a Discord notification.
func (c *Client) CreateDiscordNotification(ctx context.Context, in DiscordNotificationInput) (*Notification, error) {
	return c.createNotificationAndFindID(ctx, func() error {
		return c.do(ctx, http.MethodPost, "notification.createDiscord", in, nil, nil)
	})
}

// CreateEmailNotification creates an Email notification.
func (c *Client) CreateEmailNotification(ctx context.Context, in EmailNotificationInput) (*Notification, error) {
	return c.createNotificationAndFindID(ctx, func() error {
		return c.do(ctx, http.MethodPost, "notification.createEmail", in, nil, nil)
	})
}

// CreateTelegramNotification creates a Telegram notification.
func (c *Client) CreateTelegramNotification(ctx context.Context, in TelegramNotificationInput) (*Notification, error) {
	return c.createNotificationAndFindID(ctx, func() error {
		return c.do(ctx, http.MethodPost, "notification.createTelegram", in, nil, nil)
	})
}

// CreateGotifyNotification creates a Gotify notification.
func (c *Client) CreateGotifyNotification(ctx context.Context, in GotifyNotificationInput) (*Notification, error) {
	return c.createNotificationAndFindID(ctx, func() error {
		return c.do(ctx, http.MethodPost, "notification.createGotify", in, nil, nil)
	})
}

// GetNotification returns a single notification by ID.
func (c *Client) GetNotification(ctx context.Context, id string) (*Notification, error) {
	var out Notification
	q := url.Values{"notificationId": {id}}
	if err := c.do(ctx, http.MethodGet, "notification.one", nil, q, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateSlackNotification updates a Slack notification. Requires both notificationId
// AND slackId (from notification.one's slack.slackId). Returns empty body on success.
func (c *Client) UpdateSlackNotification(ctx context.Context, notificationID, slackID string, in SlackNotificationInput) error {
	payload := struct {
		SlackNotificationInput
		NotificationID string `json:"notificationId"`
		SlackID        string `json:"slackId"`
	}{SlackNotificationInput: in, NotificationID: notificationID, SlackID: slackID}
	return c.do(ctx, http.MethodPost, "notification.updateSlack", payload, nil, nil)
}

// UpdateDiscordNotification updates a Discord notification.
func (c *Client) UpdateDiscordNotification(ctx context.Context, notificationID, discordID string, in DiscordNotificationInput) error {
	payload := struct {
		DiscordNotificationInput
		NotificationID string `json:"notificationId"`
		DiscordID      string `json:"discordId"`
	}{DiscordNotificationInput: in, NotificationID: notificationID, DiscordID: discordID}
	return c.do(ctx, http.MethodPost, "notification.updateDiscord", payload, nil, nil)
}

// UpdateEmailNotification updates an Email notification.
func (c *Client) UpdateEmailNotification(ctx context.Context, notificationID, emailID string, in EmailNotificationInput) error {
	payload := struct {
		EmailNotificationInput
		NotificationID string `json:"notificationId"`
		EmailID        string `json:"emailId"`
	}{EmailNotificationInput: in, NotificationID: notificationID, EmailID: emailID}
	return c.do(ctx, http.MethodPost, "notification.updateEmail", payload, nil, nil)
}

// UpdateTelegramNotification updates a Telegram notification.
func (c *Client) UpdateTelegramNotification(ctx context.Context, notificationID, telegramID string, in TelegramNotificationInput) error {
	payload := struct {
		TelegramNotificationInput
		NotificationID string `json:"notificationId"`
		TelegramID     string `json:"telegramId"`
	}{TelegramNotificationInput: in, NotificationID: notificationID, TelegramID: telegramID}
	return c.do(ctx, http.MethodPost, "notification.updateTelegram", payload, nil, nil)
}

// UpdateGotifyNotification updates a Gotify notification.
func (c *Client) UpdateGotifyNotification(ctx context.Context, notificationID, gotifyID string, in GotifyNotificationInput) error {
	payload := struct {
		GotifyNotificationInput
		NotificationID string `json:"notificationId"`
		GotifyID       string `json:"gotifyId"`
	}{GotifyNotificationInput: in, NotificationID: notificationID, GotifyID: gotifyID}
	return c.do(ctx, http.MethodPost, "notification.updateGotify", payload, nil, nil)
}

// DeleteNotification deletes a notification. Uses notification.remove (not .delete).
func (c *Client) DeleteNotification(ctx context.Context, id string) error {
	payload := map[string]string{"notificationId": id}
	return c.do(ctx, http.MethodPost, "notification.remove", payload, nil, nil)
}
