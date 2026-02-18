ALTER TABLE workspaces
    DROP COLUMN IF EXISTS installed_scopes,
    DROP COLUMN IF EXISTS installed_by_user_id,
    DROP COLUMN IF EXISTS slack_bot_user_id,
    DROP COLUMN IF EXISTS slack_bot_token;
