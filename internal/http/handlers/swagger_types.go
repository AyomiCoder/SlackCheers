package handlers

import "slackcheers/internal/domain"

type ErrorResponse struct {
	Error string `json:"error"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

type HealthResponse struct {
	Status string `json:"status"`
}

type BootstrapWorkspaceRequest struct {
	SlackTeamID string `json:"slack_team_id" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Timezone    string `json:"timezone" binding:"required"`
	ChannelID   string `json:"channel_id" binding:"required"`
	ChannelName string `json:"channel_name" binding:"required"`
	PostingTime string `json:"posting_time" binding:"required"`
}

type BootstrapWorkspaceResponse struct {
	Workspace domain.Workspace        `json:"workspace"`
	Channel   domain.WorkspaceChannel `json:"channel"`
}

type UpsertPersonRequest struct {
	SlackHandle            string `json:"slack_handle" binding:"required"`
	DisplayName            string `json:"display_name" binding:"required"`
	AvatarURL              string `json:"avatar_url"`
	BirthdayDay            *int   `json:"birthday_day"`
	BirthdayMonth          *int   `json:"birthday_month"`
	BirthdayYear           *int   `json:"birthday_year"`
	HireDate               string `json:"hire_date"`
	PublicCelebrationOptIn *bool  `json:"public_celebration_opt_in"`
	RemindersMode          string `json:"reminders_mode"`
}

type UpdateChannelSettingsRequest struct {
	PostingTime          string `json:"posting_time" binding:"required"`
	Timezone             string `json:"timezone" binding:"required"`
	BirthdaysEnabled     *bool  `json:"birthdays_enabled" binding:"required"`
	AnniversariesEnabled *bool  `json:"anniversaries_enabled" binding:"required"`
}

type UpdateChannelTemplatesRequest struct {
	BirthdayTemplate    string `json:"birthday_template" binding:"required"`
	AnniversaryTemplate string `json:"anniversary_template" binding:"required"`
	BrandingEmoji       string `json:"branding_emoji"`
}

type OverviewResponse struct {
	Items []domain.UpcomingCelebration `json:"items"`
}

type PeopleResponse struct {
	People []domain.Person `json:"people"`
}

type ChannelsResponse struct {
	Channels []domain.WorkspaceChannel `json:"channels"`
}

type SlackInstallURLResponse struct {
	InstallURL string `json:"install_url"`
	State      string `json:"state"`
}

type SlackOAuthInstallation struct {
	WorkspaceID string `json:"workspace_id"`
	TeamID      string `json:"team_id"`
	TeamName    string `json:"team_name"`
	BotUserID   string `json:"bot_user_id"`
	Scope       string `json:"scope"`
}

type SlackConnectResponse struct {
	Status       string                 `json:"status"`
	Installation SlackOAuthInstallation `json:"installation"`
}

type SlackEventEnvelope struct {
	Type      string         `json:"type"`
	Token     string         `json:"token"`
	TeamID    string         `json:"team_id"`
	Challenge string         `json:"challenge"`
	Event     map[string]any `json:"event"`
}

type SlackEventAckResponse struct {
	OK        bool   `json:"ok,omitempty"`
	Challenge string `json:"challenge,omitempty"`
}

type SlackChannelItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsPrivate bool   `json:"is_private"`
}

type SlackChannelsResponse struct {
	Channels []SlackChannelItem `json:"channels"`
}

type OnboardingDMDispatchResponse struct {
	TotalMembers  int               `json:"total_members"`
	Sent          int               `json:"sent"`
	Skipped       int               `json:"skipped"`
	Failed        int               `json:"failed"`
	FailedUsers   []string          `json:"failed_users"`
	FailedDetails map[string]string `json:"failed_details"`
}

type DMCleanupResponse struct {
	UserID        string            `json:"user_id"`
	ChannelID     string            `json:"channel_id"`
	TotalMessages int               `json:"total_messages"`
	BotMessages   int               `json:"bot_messages"`
	Deleted       int               `json:"deleted"`
	Failed        int               `json:"failed"`
	FailedTS      []string          `json:"failed_ts"`
	FailedDetails map[string]string `json:"failed_details"`
}

type ManualCelebrationDispatchResponse struct {
	WorkspaceID        string                               `json:"workspace_id"`
	ChannelsProcessed  int                                  `json:"channels_processed"`
	BirthdayPosts      int                                  `json:"birthday_posts"`
	AnniversaryPosts   int                                  `json:"anniversary_posts"`
	ChannelsWithErrors int                                  `json:"channels_with_errors"`
	ChannelDispatches  []ManualCelebrationChannelDispatches `json:"channel_dispatches"`
}

type ManualCelebrationChannelDispatches struct {
	ChannelID         string `json:"channel_id"`
	SlackChannelID    string `json:"slack_channel_id"`
	BirthdayCount     int    `json:"birthday_count"`
	AnniversaryCount  int    `json:"anniversary_count"`
	BirthdayPosted    bool   `json:"birthday_posted"`
	AnniversaryPosted bool   `json:"anniversary_posted"`
	Error             string `json:"error,omitempty"`
}

type ChannelBirthdayCleanupResponse struct {
	ChannelID      string            `json:"channel_id"`
	SlackChannelID string            `json:"slack_channel_id"`
	Match          string            `json:"match"`
	Scanned        int               `json:"scanned"`
	Matched        int               `json:"matched"`
	Deleted        int               `json:"deleted"`
	Failed         int               `json:"failed"`
	FailedTS       []string          `json:"failed_ts"`
	FailedDetails  map[string]string `json:"failed_details"`
}
