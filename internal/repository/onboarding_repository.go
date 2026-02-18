package repository

import (
	"context"
	"database/sql"
	"fmt"
)

type OnboardingRepository struct {
	db *sql.DB
}

func NewOnboardingRepository(db *sql.DB) *OnboardingRepository {
	return &OnboardingRepository{db: db}
}

func (r *OnboardingRepository) ListSentUserIDs(ctx context.Context, workspaceID string) (map[string]struct{}, error) {
	const q = `
SELECT slack_user_id
FROM onboarding_dm_log
WHERE workspace_id = $1
`

	rows, err := r.db.QueryContext(ctx, q, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list onboarding dm sent users: %w", err)
	}
	defer rows.Close()

	result := make(map[string]struct{})
	for rows.Next() {
		var userID string
		if err := rows.Scan(&userID); err != nil {
			return nil, fmt.Errorf("scan onboarding dm sent user: %w", err)
		}
		result[userID] = struct{}{}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate onboarding dm sent users: %w", err)
	}

	return result, nil
}

func (r *OnboardingRepository) MarkSent(ctx context.Context, workspaceID, slackUserID string) error {
	const q = `
INSERT INTO onboarding_dm_log (workspace_id, slack_user_id)
VALUES ($1, $2)
ON CONFLICT (workspace_id, slack_user_id) DO NOTHING
`

	if _, err := r.db.ExecContext(ctx, q, workspaceID, slackUserID); err != nil {
		return fmt.Errorf("mark onboarding dm sent: %w", err)
	}
	return nil
}
