ALTER TABLE workspaces
    ADD COLUMN IF NOT EXISTS slack_bot_token TEXT,
    ADD COLUMN IF NOT EXISTS slack_bot_user_id TEXT,
    ADD COLUMN IF NOT EXISTS installed_by_user_id TEXT,
    ADD COLUMN IF NOT EXISTS installed_scopes TEXT;
