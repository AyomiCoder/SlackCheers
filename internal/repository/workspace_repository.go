package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"slackcheers/internal/domain"
)

type WorkspaceRepository struct {
	db *sql.DB
}

type WorkspaceSlackInstallation struct {
	WorkspaceID string
	SlackTeamID string
	BotToken    string
	BotUserID   string
}

type SaveSlackInstallationInput struct {
	TeamID          string
	TeamName        string
	BotToken        string
	BotUserID       string
	InstallerUserID string
	Scope           string
}

func NewWorkspaceRepository(db *sql.DB) *WorkspaceRepository {
	return &WorkspaceRepository{db: db}
}

func (r *WorkspaceRepository) EnsureWorkspace(ctx context.Context, slackTeamID, name, timezone string) (domain.Workspace, error) {
	const q = `
INSERT INTO workspaces (slack_team_id, name, timezone)
VALUES ($1, $2, $3)
ON CONFLICT (slack_team_id)
DO UPDATE SET name = EXCLUDED.name, timezone = EXCLUDED.timezone, updated_at = NOW()
RETURNING id, slack_team_id, name, timezone, created_at, updated_at
`

	var w domain.Workspace
	if err := r.db.QueryRowContext(ctx, q, slackTeamID, name, timezone).Scan(
		&w.ID,
		&w.SlackTeamID,
		&w.Name,
		&w.Timezone,
		&w.CreatedAt,
		&w.UpdatedAt,
	); err != nil {
		return domain.Workspace{}, fmt.Errorf("ensure workspace: %w", err)
	}

	return w, nil
}

func (r *WorkspaceRepository) EnsureWorkspaceFromInstall(ctx context.Context, slackTeamID, name string) (domain.Workspace, error) {
	const q = `
INSERT INTO workspaces (slack_team_id, name, timezone)
VALUES ($1, $2, 'UTC')
ON CONFLICT (slack_team_id)
DO UPDATE SET name = EXCLUDED.name, updated_at = NOW()
RETURNING id, slack_team_id, name, timezone, created_at, updated_at
`

	var w domain.Workspace
	if err := r.db.QueryRowContext(ctx, q, slackTeamID, name).Scan(
		&w.ID,
		&w.SlackTeamID,
		&w.Name,
		&w.Timezone,
		&w.CreatedAt,
		&w.UpdatedAt,
	); err != nil {
		return domain.Workspace{}, fmt.Errorf("ensure workspace from install: %w", err)
	}

	return w, nil
}

func (r *WorkspaceRepository) SaveSlackInstallation(ctx context.Context, in SaveSlackInstallationInput) (domain.Workspace, error) {
	workspace, err := r.EnsureWorkspaceFromInstall(ctx, in.TeamID, in.TeamName)
	if err != nil {
		return domain.Workspace{}, err
	}

	const q = `
UPDATE workspaces
SET slack_bot_token = $2,
    slack_bot_user_id = $3,
    installed_by_user_id = $4,
    installed_scopes = $5,
    updated_at = NOW()
WHERE id = $1
`
	if _, err := r.db.ExecContext(
		ctx,
		q,
		workspace.ID,
		in.BotToken,
		in.BotUserID,
		in.InstallerUserID,
		in.Scope,
	); err != nil {
		return domain.Workspace{}, fmt.Errorf("save slack installation: %w", err)
	}

	return workspace, nil
}

func (r *WorkspaceRepository) GetSlackInstallationByWorkspaceID(ctx context.Context, workspaceID string) (WorkspaceSlackInstallation, error) {
	const q = `
SELECT id, slack_team_id, COALESCE(slack_bot_token, ''), COALESCE(slack_bot_user_id, '')
FROM workspaces
WHERE id = $1
`

	var out WorkspaceSlackInstallation
	if err := r.db.QueryRowContext(ctx, q, workspaceID).Scan(&out.WorkspaceID, &out.SlackTeamID, &out.BotToken, &out.BotUserID); err != nil {
		if err == sql.ErrNoRows {
			return WorkspaceSlackInstallation{}, ErrNotFound
		}
		return WorkspaceSlackInstallation{}, fmt.Errorf("get workspace slack installation: %w", err)
	}

	return out, nil
}

func (r *WorkspaceRepository) GetSlackInstallationByTeamID(ctx context.Context, slackTeamID string) (WorkspaceSlackInstallation, error) {
	const q = `
SELECT id, slack_team_id, COALESCE(slack_bot_token, ''), COALESCE(slack_bot_user_id, '')
FROM workspaces
WHERE slack_team_id = $1
`

	var out WorkspaceSlackInstallation
	if err := r.db.QueryRowContext(ctx, q, slackTeamID).Scan(&out.WorkspaceID, &out.SlackTeamID, &out.BotToken, &out.BotUserID); err != nil {
		if err == sql.ErrNoRows {
			return WorkspaceSlackInstallation{}, ErrNotFound
		}
		return WorkspaceSlackInstallation{}, fmt.Errorf("get workspace slack installation by team id: %w", err)
	}

	return out, nil
}

func (r *WorkspaceRepository) CreateDefaultChannel(ctx context.Context, workspaceID, channelID, channelName, timezone, postingTime string) (domain.WorkspaceChannel, error) {
	const q = `
INSERT INTO workspace_channels (
    workspace_id, slack_channel_id, slack_channel_name, posting_time, timezone
)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (workspace_id, slack_channel_id)
DO UPDATE SET
    slack_channel_name = EXCLUDED.slack_channel_name,
    posting_time = EXCLUDED.posting_time,
    timezone = EXCLUDED.timezone,
    updated_at = NOW()
RETURNING id, workspace_id, slack_channel_id, slack_channel_name,
          to_char(posting_time, 'HH24:MI'), timezone,
          birthdays_enabled, anniversaries_enabled,
          birthday_template, anniversary_template, COALESCE(branding_emoji, ''),
          created_at, updated_at
`

	var c domain.WorkspaceChannel
	if err := r.db.QueryRowContext(ctx, q, workspaceID, channelID, channelName, postingTime, timezone).Scan(
		&c.ID,
		&c.WorkspaceID,
		&c.SlackChannelID,
		&c.SlackChannelName,
		&c.PostingTime,
		&c.Timezone,
		&c.BirthdaysEnabled,
		&c.AnniversariesEnabled,
		&c.BirthdayTemplate,
		&c.AnniversaryTemplate,
		&c.BrandingEmoji,
		&c.CreatedAt,
		&c.UpdatedAt,
	); err != nil {
		return domain.WorkspaceChannel{}, fmt.Errorf("create or update channel: %w", err)
	}

	return c, nil
}

func (r *WorkspaceRepository) ListChannelsByWorkspace(ctx context.Context, workspaceID string) ([]domain.WorkspaceChannel, error) {
	const q = `
SELECT id, workspace_id, slack_channel_id, slack_channel_name,
       to_char(posting_time, 'HH24:MI'), timezone,
       birthdays_enabled, anniversaries_enabled,
       birthday_template, anniversary_template, COALESCE(branding_emoji, ''),
       created_at, updated_at
FROM workspace_channels
WHERE workspace_id = $1
ORDER BY slack_channel_name
`

	rows, err := r.db.QueryContext(ctx, q, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list channels: %w", err)
	}
	defer rows.Close()

	channels := make([]domain.WorkspaceChannel, 0)
	for rows.Next() {
		var c domain.WorkspaceChannel
		if err := rows.Scan(
			&c.ID,
			&c.WorkspaceID,
			&c.SlackChannelID,
			&c.SlackChannelName,
			&c.PostingTime,
			&c.Timezone,
			&c.BirthdaysEnabled,
			&c.AnniversariesEnabled,
			&c.BirthdayTemplate,
			&c.AnniversaryTemplate,
			&c.BrandingEmoji,
			&c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan channel: %w", err)
		}
		channels = append(channels, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate channels: %w", err)
	}

	return channels, nil
}

func (r *WorkspaceRepository) UpdateChannelSettings(ctx context.Context, workspaceID, channelID, postingTime, timezone string, birthdaysEnabled, anniversariesEnabled bool) (domain.WorkspaceChannel, error) {
	const q = `
UPDATE workspace_channels
SET posting_time = $3,
    timezone = $4,
    birthdays_enabled = $5,
    anniversaries_enabled = $6,
    updated_at = NOW()
WHERE workspace_id = $1
  AND (id::text = $2 OR slack_channel_id = $2)
RETURNING id, workspace_id, slack_channel_id, slack_channel_name,
          to_char(posting_time, 'HH24:MI'), timezone,
          birthdays_enabled, anniversaries_enabled,
          birthday_template, anniversary_template, COALESCE(branding_emoji, ''),
          created_at, updated_at
`

	var c domain.WorkspaceChannel
	if err := r.db.QueryRowContext(ctx, q, workspaceID, channelID, postingTime, timezone, birthdaysEnabled, anniversariesEnabled).Scan(
		&c.ID,
		&c.WorkspaceID,
		&c.SlackChannelID,
		&c.SlackChannelName,
		&c.PostingTime,
		&c.Timezone,
		&c.BirthdaysEnabled,
		&c.AnniversariesEnabled,
		&c.BirthdayTemplate,
		&c.AnniversaryTemplate,
		&c.BrandingEmoji,
		&c.CreatedAt,
		&c.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return domain.WorkspaceChannel{}, ErrNotFound
		}
		return domain.WorkspaceChannel{}, fmt.Errorf("update channel settings: %w", err)
	}

	return c, nil
}

func (r *WorkspaceRepository) UpdateChannelTemplates(ctx context.Context, workspaceID, channelID, birthdayTemplate, anniversaryTemplate, brandingEmoji string) (domain.WorkspaceChannel, error) {
	const q = `
UPDATE workspace_channels
SET birthday_template = $3,
    anniversary_template = $4,
    branding_emoji = $5,
    updated_at = NOW()
WHERE workspace_id = $1
  AND (id::text = $2 OR slack_channel_id = $2)
RETURNING id, workspace_id, slack_channel_id, slack_channel_name,
          to_char(posting_time, 'HH24:MI'), timezone,
          birthdays_enabled, anniversaries_enabled,
          birthday_template, anniversary_template, COALESCE(branding_emoji, ''),
          created_at, updated_at
`

	var c domain.WorkspaceChannel
	if err := r.db.QueryRowContext(ctx, q, workspaceID, channelID, birthdayTemplate, anniversaryTemplate, brandingEmoji).Scan(
		&c.ID,
		&c.WorkspaceID,
		&c.SlackChannelID,
		&c.SlackChannelName,
		&c.PostingTime,
		&c.Timezone,
		&c.BirthdaysEnabled,
		&c.AnniversariesEnabled,
		&c.BirthdayTemplate,
		&c.AnniversaryTemplate,
		&c.BrandingEmoji,
		&c.CreatedAt,
		&c.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return domain.WorkspaceChannel{}, ErrNotFound
		}
		return domain.WorkspaceChannel{}, fmt.Errorf("update channel templates: %w", err)
	}

	return c, nil
}

func (r *WorkspaceRepository) ListDueChannels(ctx context.Context, now time.Time) ([]domain.WorkspaceChannel, error) {
	const q = `
SELECT wc.id, wc.workspace_id, wc.slack_channel_id, wc.slack_channel_name,
       to_char(wc.posting_time, 'HH24:MI'), wc.timezone,
       wc.birthdays_enabled, wc.anniversaries_enabled,
       wc.birthday_template, wc.anniversary_template, COALESCE(wc.branding_emoji, ''),
       wc.created_at, wc.updated_at
FROM workspace_channels wc
WHERE EXTRACT(HOUR FROM timezone(wc.timezone, $1)) = EXTRACT(HOUR FROM wc.posting_time)
  AND EXTRACT(MINUTE FROM timezone(wc.timezone, $1)) = EXTRACT(MINUTE FROM wc.posting_time)
  AND NOT EXISTS (
      SELECT 1
      FROM celebration_dispatch_log l
      WHERE l.workspace_channel_id = wc.id
        AND l.dispatch_date = (timezone(wc.timezone, $1))::date
  )
`

	rows, err := r.db.QueryContext(ctx, q, now.UTC())
	if err != nil {
		return nil, fmt.Errorf("list due channels: %w", err)
	}
	defer rows.Close()

	channels := make([]domain.WorkspaceChannel, 0)
	for rows.Next() {
		var c domain.WorkspaceChannel
		if err := rows.Scan(
			&c.ID,
			&c.WorkspaceID,
			&c.SlackChannelID,
			&c.SlackChannelName,
			&c.PostingTime,
			&c.Timezone,
			&c.BirthdaysEnabled,
			&c.AnniversariesEnabled,
			&c.BirthdayTemplate,
			&c.AnniversaryTemplate,
			&c.BrandingEmoji,
			&c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan due channel: %w", err)
		}
		channels = append(channels, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate due channels: %w", err)
	}

	return channels, nil
}

func (r *WorkspaceRepository) MarkChannelDispatched(ctx context.Context, channelID string, dispatchDate time.Time) error {
	const q = `
INSERT INTO celebration_dispatch_log (workspace_channel_id, dispatch_date)
VALUES ($1, $2)
ON CONFLICT (workspace_channel_id, dispatch_date) DO NOTHING
`

	if _, err := r.db.ExecContext(ctx, q, channelID, dispatchDate.Format("2006-01-02")); err != nil {
		return fmt.Errorf("mark channel dispatched: %w", err)
	}

	return nil
}
