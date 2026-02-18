package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"slackcheers/internal/domain"
)

type UpsertPersonInput struct {
	WorkspaceID            string
	SlackUserID            string
	SlackHandle            string
	DisplayName            string
	AvatarURL              string
	BirthdayDay            *int
	BirthdayMonth          *int
	BirthdayYear           *int
	HireDate               *time.Time
	PublicCelebrationOptIn bool
	RemindersMode          string
}

type PeopleRepository struct {
	db *sql.DB
}

func NewPeopleRepository(db *sql.DB) *PeopleRepository {
	return &PeopleRepository{db: db}
}

func (r *PeopleRepository) ListByWorkspace(ctx context.Context, workspaceID string) ([]domain.Person, error) {
	const q = `
SELECT id, workspace_id, slack_user_id, slack_handle, display_name, avatar_url,
       birthday_day, birthday_month, birthday_year,
       hire_date, public_celebration_opt_in, reminders_mode, created_at, updated_at
FROM people
WHERE workspace_id = $1
ORDER BY display_name
`

	rows, err := r.db.QueryContext(ctx, q, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list people: %w", err)
	}
	defer rows.Close()

	people := make([]domain.Person, 0)
	for rows.Next() {
		p, err := scanPerson(rows)
		if err != nil {
			return nil, err
		}
		people = append(people, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate people: %w", err)
	}

	return people, nil
}

func (r *PeopleRepository) Upsert(ctx context.Context, in UpsertPersonInput) (domain.Person, error) {
	const q = `
INSERT INTO people (
    workspace_id, slack_user_id, slack_handle, display_name, avatar_url,
    birthday_day, birthday_month, birthday_year, hire_date,
    public_celebration_opt_in, reminders_mode
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (workspace_id, slack_user_id)
DO UPDATE SET
    slack_handle = EXCLUDED.slack_handle,
    display_name = EXCLUDED.display_name,
    avatar_url = EXCLUDED.avatar_url,
    birthday_day = EXCLUDED.birthday_day,
    birthday_month = EXCLUDED.birthday_month,
    birthday_year = EXCLUDED.birthday_year,
    hire_date = EXCLUDED.hire_date,
    public_celebration_opt_in = EXCLUDED.public_celebration_opt_in,
    reminders_mode = EXCLUDED.reminders_mode,
    updated_at = NOW()
RETURNING id, workspace_id, slack_user_id, slack_handle, display_name, avatar_url,
          birthday_day, birthday_month, birthday_year,
          hire_date, public_celebration_opt_in, reminders_mode, created_at, updated_at
`

	var hireDate sql.NullTime
	if in.HireDate != nil {
		hireDate.Valid = true
		hireDate.Time = *in.HireDate
	}

	row := r.db.QueryRowContext(
		ctx,
		q,
		in.WorkspaceID,
		in.SlackUserID,
		in.SlackHandle,
		in.DisplayName,
		in.AvatarURL,
		toNullInt16(in.BirthdayDay),
		toNullInt16(in.BirthdayMonth),
		toNullInt16(in.BirthdayYear),
		hireDate,
		in.PublicCelebrationOptIn,
		in.RemindersMode,
	)

	p, err := scanPerson(row)
	if err != nil {
		return domain.Person{}, fmt.Errorf("upsert person: %w", err)
	}

	return p, nil
}

func (r *PeopleRepository) FindBirthdaysByWorkspaceAndDate(ctx context.Context, workspaceID string, month, day int) ([]domain.Person, error) {
	const q = `
SELECT id, workspace_id, slack_user_id, slack_handle, display_name, avatar_url,
       birthday_day, birthday_month, birthday_year,
       hire_date, public_celebration_opt_in, reminders_mode, created_at, updated_at
FROM people
WHERE workspace_id = $1
  AND public_celebration_opt_in = TRUE
  AND birthday_month = $2
  AND birthday_day = $3
ORDER BY display_name
`

	rows, err := r.db.QueryContext(ctx, q, workspaceID, month, day)
	if err != nil {
		return nil, fmt.Errorf("find birthdays: %w", err)
	}
	defer rows.Close()

	birthdays := make([]domain.Person, 0)
	for rows.Next() {
		p, err := scanPerson(rows)
		if err != nil {
			return nil, err
		}
		birthdays = append(birthdays, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate birthdays: %w", err)
	}

	return birthdays, nil
}

func (r *PeopleRepository) FindAnniversariesByWorkspaceAndDate(ctx context.Context, workspaceID string, month, day, year int) ([]domain.AnniversaryPerson, error) {
	const q = `
SELECT id, workspace_id, slack_user_id, slack_handle, display_name, avatar_url,
       birthday_day, birthday_month, birthday_year,
       hire_date, public_celebration_opt_in, reminders_mode,
       created_at, updated_at,
       ($4 - EXTRACT(YEAR FROM hire_date)::int) AS years
FROM people
WHERE workspace_id = $1
  AND public_celebration_opt_in = TRUE
  AND hire_date IS NOT NULL
  AND EXTRACT(MONTH FROM hire_date) = $2
  AND EXTRACT(DAY FROM hire_date) = $3
ORDER BY display_name
`

	rows, err := r.db.QueryContext(ctx, q, workspaceID, month, day, year)
	if err != nil {
		return nil, fmt.Errorf("find anniversaries: %w", err)
	}
	defer rows.Close()

	results := make([]domain.AnniversaryPerson, 0)
	for rows.Next() {
		var p domain.Person
		var years int
		if err := scanPersonWithYears(rows, &p, &years); err != nil {
			return nil, err
		}

		results = append(results, domain.AnniversaryPerson{Person: p, Years: years})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate anniversaries: %w", err)
	}

	return results, nil
}

func toNullInt16(v *int) sql.NullInt16 {
	if v == nil {
		return sql.NullInt16{}
	}
	return sql.NullInt16{Int16: int16(*v), Valid: true}
}

type personScanner interface {
	Scan(dest ...any) error
}

func scanPerson(scanner personScanner) (domain.Person, error) {
	var (
		p             domain.Person
		birthdayDay   sql.NullInt16
		birthdayMonth sql.NullInt16
		birthdayYear  sql.NullInt16
		hireDate      sql.NullTime
	)

	if err := scanner.Scan(
		&p.ID,
		&p.WorkspaceID,
		&p.SlackUserID,
		&p.SlackHandle,
		&p.DisplayName,
		&p.AvatarURL,
		&birthdayDay,
		&birthdayMonth,
		&birthdayYear,
		&hireDate,
		&p.PublicCelebrationOptIn,
		&p.RemindersMode,
		&p.CreatedAt,
		&p.UpdatedAt,
	); err != nil {
		return domain.Person{}, fmt.Errorf("scan person: %w", err)
	}

	if birthdayDay.Valid {
		v := int(birthdayDay.Int16)
		p.BirthdayDay = &v
	}
	if birthdayMonth.Valid {
		v := int(birthdayMonth.Int16)
		p.BirthdayMonth = &v
	}
	if birthdayYear.Valid {
		v := int(birthdayYear.Int16)
		p.BirthdayYear = &v
	}
	if hireDate.Valid {
		v := hireDate.Time
		p.HireDate = &v
	}

	return p, nil
}

func scanPersonWithYears(scanner personScanner, p *domain.Person, years *int) error {
	var (
		birthdayDay   sql.NullInt16
		birthdayMonth sql.NullInt16
		birthdayYear  sql.NullInt16
		hireDate      sql.NullTime
	)

	if err := scanner.Scan(
		&p.ID,
		&p.WorkspaceID,
		&p.SlackUserID,
		&p.SlackHandle,
		&p.DisplayName,
		&p.AvatarURL,
		&birthdayDay,
		&birthdayMonth,
		&birthdayYear,
		&hireDate,
		&p.PublicCelebrationOptIn,
		&p.RemindersMode,
		&p.CreatedAt,
		&p.UpdatedAt,
		years,
	); err != nil {
		return fmt.Errorf("scan anniversary person: %w", err)
	}

	if birthdayDay.Valid {
		v := int(birthdayDay.Int16)
		p.BirthdayDay = &v
	}
	if birthdayMonth.Valid {
		v := int(birthdayMonth.Int16)
		p.BirthdayMonth = &v
	}
	if birthdayYear.Valid {
		v := int(birthdayYear.Int16)
		p.BirthdayYear = &v
	}
	if hireDate.Valid {
		v := hireDate.Time
		p.HireDate = &v
	}

	return nil
}
