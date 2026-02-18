CREATE TABLE IF NOT EXISTS onboarding_dm_log (
    id BIGSERIAL PRIMARY KEY,
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    slack_user_id TEXT NOT NULL,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (workspace_id, slack_user_id)
);

CREATE INDEX IF NOT EXISTS idx_onboarding_dm_log_workspace ON onboarding_dm_log(workspace_id);
