CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS workspaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slack_team_id TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    timezone TEXT NOT NULL DEFAULT 'UTC',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS workspace_channels (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    slack_channel_id TEXT NOT NULL,
    slack_channel_name TEXT NOT NULL,
    posting_time TIME NOT NULL DEFAULT '09:00:00',
    timezone TEXT NOT NULL DEFAULT 'UTC',
    birthdays_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    anniversaries_enabled BOOLEAN NOT NULL DEFAULT TRUE,
    birthday_template TEXT NOT NULL DEFAULT '🎂 Happy birthday, {users}!',
    anniversary_template TEXT NOT NULL DEFAULT '🎉 Happy {years}-year anniversary, {users}!',
    branding_emoji TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(workspace_id, slack_channel_id)
);

CREATE TABLE IF NOT EXISTS people (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    slack_user_id TEXT NOT NULL,
    slack_handle TEXT NOT NULL,
    display_name TEXT NOT NULL,
    avatar_url TEXT,
    birthday_day SMALLINT CHECK (birthday_day BETWEEN 1 AND 31),
    birthday_month SMALLINT CHECK (birthday_month BETWEEN 1 AND 12),
    birthday_year SMALLINT CHECK (birthday_year BETWEEN 1900 AND 3000),
    hire_date DATE,
    public_celebration_opt_in BOOLEAN NOT NULL DEFAULT TRUE,
    reminders_mode TEXT NOT NULL DEFAULT 'same_day' CHECK (reminders_mode IN ('none', 'same_day', 'day_before')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(workspace_id, slack_user_id)
);

CREATE TABLE IF NOT EXISTS celebration_dispatch_log (
    id BIGSERIAL PRIMARY KEY,
    workspace_channel_id UUID NOT NULL REFERENCES workspace_channels(id) ON DELETE CASCADE,
    dispatch_date DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(workspace_channel_id, dispatch_date)
);

CREATE INDEX IF NOT EXISTS idx_people_workspace ON people(workspace_id);
CREATE INDEX IF NOT EXISTS idx_people_birthday ON people(workspace_id, birthday_month, birthday_day);
CREATE INDEX IF NOT EXISTS idx_people_hire_date ON people(workspace_id, hire_date);
CREATE INDEX IF NOT EXISTS idx_workspace_channels_workspace ON workspace_channels(workspace_id);
