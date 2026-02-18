package domain

import "time"

type Workspace struct {
	ID                   string
	SlackTeamID          string
	Name                 string
	Timezone             string
	BirthdaysEnabled     bool
	AnniversariesEnabled bool
	DefaultTemplateStyle string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type WorkspaceChannel struct {
	ID                   string
	WorkspaceID          string
	SlackChannelID       string
	SlackChannelName     string
	PostingTime          string
	Timezone             string
	BirthdaysEnabled     bool
	AnniversariesEnabled bool
	BirthdayTemplate     string
	AnniversaryTemplate  string
	BrandingEmoji        string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type Person struct {
	ID                     string
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
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

type UpcomingCelebration struct {
	Date      time.Time
	Type      string
	UserID    string
	SlackUser string
	Name      string
	Years     *int
}

type DailyCelebrationPayload struct {
	WorkspaceChannel WorkspaceChannel
	Birthdays        []Person
	Anniversaries    []AnniversaryPerson
}

type AnniversaryPerson struct {
	Person
	Years int
}
