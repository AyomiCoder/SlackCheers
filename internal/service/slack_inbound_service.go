package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"slackcheers/internal/repository"
	"slackcheers/internal/slack"
)

const slackUsersInfoURL = "https://slack.com/api/users.info"

type SlackInboundService struct {
	workspaceRepo *repository.WorkspaceRepository
	peopleRepo    *repository.PeopleRepository
	slackClient   slack.Client
	logger        *slog.Logger
	httpClient    *http.Client
}

type inboundEventEnvelope struct {
	Type   string `json:"type"`
	TeamID string `json:"team_id"`
	Event  struct {
		Type        string `json:"type"`
		Subtype     string `json:"subtype"`
		BotID       string `json:"bot_id"`
		User        string `json:"user"`
		Text        string `json:"text"`
		ChannelType string `json:"channel_type"`
	} `json:"event"`
}

type slackUsersInfoResponse struct {
	OK       bool   `json:"ok"`
	Error    string `json:"error"`
	Needed   string `json:"needed"`
	Provided string `json:"provided"`
	User     struct {
		Name    string `json:"name"`
		Profile struct {
			DisplayName string `json:"display_name"`
			RealName    string `json:"real_name"`
			Image192    string `json:"image_192"`
		} `json:"profile"`
	} `json:"user"`
}

type parsedProfileInput struct {
	HasBirthday bool
	BirthdayDay int
	BirthdayMon int
	BirthdayYr  *int
	HasHireDate bool
	HireDate    time.Time
}

var (
	namedDatePattern = regexp.MustCompile(`(?i)^/?\s*([a-z]+)\s+([0-3]?\d)(?:\s*,\s*(\d{4}))?\s*$`)
	birthdayPattern  = regexp.MustCompile(`(?i)\bbirthday\b\s*[:=-]?\s*([0-3]?\d)[/.-]([01]?\d)(?:[/.-](\d{4}))?`)
	hirePattern      = regexp.MustCompile(`(?i)\b(?:hire[_ ]?date|start[_ ]?date|work[_ ]?start)\b\s*[:=-]?\s*(\d{4}-\d{2}-\d{2})`)
	onlyBirthday     = regexp.MustCompile(`^\s*([0-3]?\d)[/.-]([01]?\d)(?:[/.-](\d{4}))?\s*$`)
	onlyHireDate     = regexp.MustCompile(`^\s*(\d{4}-\d{2}-\d{2})\s*$`)
)

var monthNames = map[string]int{
	"jan":       1,
	"january":   1,
	"feb":       2,
	"february":  2,
	"mar":       3,
	"march":     3,
	"apr":       4,
	"april":     4,
	"may":       5,
	"jun":       6,
	"june":      6,
	"jul":       7,
	"july":      7,
	"aug":       8,
	"august":    8,
	"sep":       9,
	"sept":      9,
	"september": 9,
	"oct":       10,
	"october":   10,
	"nov":       11,
	"november":  11,
	"dec":       12,
	"december":  12,
}

func NewSlackInboundService(
	workspaceRepo *repository.WorkspaceRepository,
	peopleRepo *repository.PeopleRepository,
	slackClient slack.Client,
	logger *slog.Logger,
) *SlackInboundService {
	return &SlackInboundService{
		workspaceRepo: workspaceRepo,
		peopleRepo:    peopleRepo,
		slackClient:   slackClient,
		logger:        logger,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *SlackInboundService) ProcessEvent(ctx context.Context, raw []byte) error {
	var envelope inboundEventEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decode inbound event payload: %w", err)
	}

	if envelope.Type != "event_callback" {
		return nil
	}

	ev := envelope.Event
	if ev.Type != "message" || ev.ChannelType != "im" || strings.TrimSpace(ev.User) == "" {
		return nil
	}
	if strings.TrimSpace(ev.Subtype) != "" || strings.TrimSpace(ev.BotID) != "" {
		return nil
	}

	install, err := s.workspaceRepo.GetSlackInstallationByTeamID(ctx, strings.TrimSpace(envelope.TeamID))
	if err != nil {
		return fmt.Errorf("resolve workspace by team id: %w", err)
	}

	parsed, err := parseProfileInput(ev.Text)
	if err != nil {
		help := buildProfileInputHelpMessage(err.Error())
		_ = s.slackClient.SendDirectMessage(ctx, install.WorkspaceID, ev.User, help)
		return nil
	}

	profile, profileErr := s.fetchSlackUserProfile(ctx, install.BotToken, ev.User)
	if profileErr != nil {
		s.logger.WarnContext(ctx, "failed to fetch slack user profile", slog.String("user_id", ev.User), slog.String("error", profileErr.Error()))
	}

	mergedInput, _, err := s.buildPersonUpsert(ctx, install.WorkspaceID, ev.User, parsed, profile)
	if err != nil {
		return err
	}

	if _, err := s.peopleRepo.Upsert(ctx, mergedInput); err != nil {
		return err
	}

	ack := buildSaveAckMessage(parsed)
	if err := s.slackClient.SendDirectMessage(ctx, install.WorkspaceID, ev.User, ack); err != nil {
		s.logger.WarnContext(ctx, "failed to send inbound save ack", slog.String("user_id", ev.User), slog.String("error", err.Error()))
	}

	return nil
}

type slackUserProfile struct {
	SlackHandle string
	DisplayName string
	AvatarURL   string
}

func (s *SlackInboundService) fetchSlackUserProfile(ctx context.Context, token, userID string) (slackUserProfile, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, slackUsersInfoURL, nil)
	if err != nil {
		return slackUserProfile{}, fmt.Errorf("build users.info request: %w", err)
	}

	q := req.URL.Query()
	q.Set("user", userID)
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return slackUserProfile{}, fmt.Errorf("call users.info: %w", err)
	}
	defer resp.Body.Close()

	var payload slackUsersInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return slackUserProfile{}, fmt.Errorf("decode users.info response: %w", err)
	}
	if !payload.OK {
		if payload.Error == "" {
			payload.Error = "users.info failed"
		}
		return slackUserProfile{}, fmt.Errorf("slack api error: %s%s", payload.Error, slackScopeHint(payload.Needed, payload.Provided))
	}

	handle := strings.TrimSpace(payload.User.Name)
	displayName := strings.TrimSpace(payload.User.Profile.DisplayName)
	if displayName == "" {
		displayName = strings.TrimSpace(payload.User.Profile.RealName)
	}

	return slackUserProfile{
		SlackHandle: handle,
		DisplayName: displayName,
		AvatarURL:   strings.TrimSpace(payload.User.Profile.Image192),
	}, nil
}

func (s *SlackInboundService) buildPersonUpsert(
	ctx context.Context,
	workspaceID, slackUserID string,
	parsed parsedProfileInput,
	profile slackUserProfile,
) (repository.UpsertPersonInput, string, error) {
	existing, err := s.peopleRepo.GetByWorkspaceAndSlackUserID(ctx, workspaceID, slackUserID)
	if err != nil && err != repository.ErrNotFound {
		return repository.UpsertPersonInput{}, "", err
	}

	in := repository.UpsertPersonInput{
		WorkspaceID:            workspaceID,
		SlackUserID:            slackUserID,
		SlackHandle:            fallbackString(profile.SlackHandle, existing.SlackHandle, slackUserID),
		DisplayName:            fallbackString(profile.DisplayName, existing.DisplayName, slackUserID),
		AvatarURL:              fallbackString(profile.AvatarURL, existing.AvatarURL, ""),
		PublicCelebrationOptIn: true,
		RemindersMode:          "same_day",
		BirthdayDay:            existing.BirthdayDay,
		BirthdayMonth:          existing.BirthdayMonth,
		BirthdayYear:           existing.BirthdayYear,
		HireDate:               existing.HireDate,
	}

	if err == nil {
		in.PublicCelebrationOptIn = existing.PublicCelebrationOptIn
		if strings.TrimSpace(existing.RemindersMode) != "" {
			in.RemindersMode = existing.RemindersMode
		}
	}

	parts := make([]string, 0, 2)
	if parsed.HasBirthday {
		day := parsed.BirthdayDay
		month := parsed.BirthdayMon
		in.BirthdayDay = &day
		in.BirthdayMonth = &month
		in.BirthdayYear = parsed.BirthdayYr
		if parsed.BirthdayYr != nil {
			parts = append(parts, fmt.Sprintf("birthday=%02d/%02d/%d", day, month, *parsed.BirthdayYr))
		} else {
			parts = append(parts, fmt.Sprintf("birthday=%02d/%02d", day, month))
		}
	}
	if parsed.HasHireDate {
		d := parsed.HireDate
		in.HireDate = &d
		parts = append(parts, "hire_date="+d.Format("2006-01-02"))
	}

	return in, strings.Join(parts, ", "), nil
}

func parseProfileInput(text string) (parsedProfileInput, error) {
	clean := strings.TrimSpace(text)
	if clean == "" {
		return parsedProfileInput{}, fmt.Errorf("empty message")
	}

	// Preferred format:
	// march 25             -> birthday
	// january 23, 2024     -> hire date (year required)
	// Optional leading "/" is accepted if provided.
	if parsed, usedNamedMode, err := parseNamedDateProfileInput(clean); usedNamedMode || err != nil {
		return parsed, err
	}

	return parseLegacyProfileInput(clean)
}

func parseNamedDateProfileInput(text string) (parsedProfileInput, bool, error) {
	parsed := parsedProfileInput{}
	lines := strings.Split(text, "\n")
	usedNamedMode := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		month, day, year, matched, err := parseNamedDateLine(line)
		if err != nil {
			return parsedProfileInput{}, true, err
		}
		if !matched {
			if usedNamedMode {
				return parsedProfileInput{}, true, fmt.Errorf("invalid date line format")
			}
			continue
		}
		usedNamedMode = true

		if year == nil {
			if parsed.HasBirthday {
				return parsedProfileInput{}, true, fmt.Errorf("multiple birthday lines provided")
			}
			parsed.HasBirthday = true
			parsed.BirthdayDay = day
			parsed.BirthdayMon = month
			parsed.BirthdayYr = nil
			continue
		}

		if parsed.HasHireDate {
			return parsedProfileInput{}, true, fmt.Errorf("multiple hire date lines provided")
		}

		hireDate := time.Date(*year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
		if int(hireDate.Month()) != month || hireDate.Day() != day {
			return parsedProfileInput{}, true, fmt.Errorf("invalid hire date")
		}
		parsed.HasHireDate = true
		parsed.HireDate = hireDate
	}

	if !usedNamedMode {
		return parsedProfileInput{}, false, nil
	}

	if !parsed.HasBirthday && !parsed.HasHireDate {
		return parsedProfileInput{}, true, fmt.Errorf("no birthday or hire date found")
	}

	return parsed, true, nil
}

func parseLegacyProfileInput(clean string) (parsedProfileInput, error) {
	parsed := parsedProfileInput{}

	if m := birthdayPattern.FindStringSubmatch(clean); len(m) >= 3 {
		day, month, yearPtr, err := parseBirthdayParts(m[1], m[2], matchOrEmpty(m, 3))
		if err != nil {
			return parsedProfileInput{}, err
		}
		parsed.HasBirthday = true
		parsed.BirthdayDay = day
		parsed.BirthdayMon = month
		parsed.BirthdayYr = yearPtr
	} else if m := onlyBirthday.FindStringSubmatch(clean); len(m) >= 3 {
		day, month, yearPtr, err := parseBirthdayParts(m[1], m[2], matchOrEmpty(m, 3))
		if err != nil {
			return parsedProfileInput{}, err
		}
		parsed.HasBirthday = true
		parsed.BirthdayDay = day
		parsed.BirthdayMon = month
		parsed.BirthdayYr = yearPtr
	}

	if m := hirePattern.FindStringSubmatch(clean); len(m) >= 2 {
		hireDate, err := time.Parse("2006-01-02", m[1])
		if err != nil {
			return parsedProfileInput{}, fmt.Errorf("invalid hire date format (use YYYY-MM-DD)")
		}
		parsed.HasHireDate = true
		parsed.HireDate = hireDate
	} else if m := onlyHireDate.FindStringSubmatch(clean); len(m) >= 2 {
		hireDate, err := time.Parse("2006-01-02", m[1])
		if err != nil {
			return parsedProfileInput{}, fmt.Errorf("invalid hire date format (use YYYY-MM-DD)")
		}
		parsed.HasHireDate = true
		parsed.HireDate = hireDate
	}

	if !parsed.HasBirthday && !parsed.HasHireDate {
		return parsedProfileInput{}, fmt.Errorf("no birthday or hire date found")
	}

	return parsed, nil
}

func parseNamedDateLine(line string) (month int, day int, year *int, matched bool, err error) {
	m := namedDatePattern.FindStringSubmatch(strings.TrimSpace(line))
	if len(m) == 0 {
		return 0, 0, nil, false, nil
	}
	if len(m) < 3 {
		return 0, 0, nil, true, fmt.Errorf("invalid format (use march 25 or january 23, 2024)")
	}

	monthRaw := strings.ToLower(strings.TrimSpace(m[1]))
	monthRaw = strings.TrimSuffix(monthRaw, ".")
	parsedMonth, ok := monthNames[monthRaw]
	if !ok {
		return 0, 0, nil, true, fmt.Errorf("invalid month name")
	}

	parsedDay, err := strconv.Atoi(strings.TrimSpace(m[2]))
	if err != nil || parsedDay < 1 || parsedDay > 31 {
		return 0, 0, nil, true, fmt.Errorf("invalid day value")
	}
	if !validDayMonth(parsedDay, parsedMonth) {
		return 0, 0, nil, true, fmt.Errorf("invalid calendar date")
	}

	var parsedYear *int
	if len(m) >= 4 && strings.TrimSpace(m[3]) != "" {
		yearValue, err := strconv.Atoi(strings.TrimSpace(m[3]))
		if err != nil || yearValue < 1900 || yearValue > 3000 {
			return 0, 0, nil, true, fmt.Errorf("invalid year value")
		}
		parsedYear = &yearValue
	}

	return parsedMonth, parsedDay, parsedYear, true, nil
}

func parseBirthdayParts(dayRaw, monthRaw, yearRaw string) (int, int, *int, error) {
	day, err := strconv.Atoi(strings.TrimSpace(dayRaw))
	if err != nil {
		return 0, 0, nil, fmt.Errorf("invalid birthday day")
	}
	month, err := strconv.Atoi(strings.TrimSpace(monthRaw))
	if err != nil {
		return 0, 0, nil, fmt.Errorf("invalid birthday month")
	}
	if day < 1 || day > 31 || month < 1 || month > 12 {
		return 0, 0, nil, fmt.Errorf("invalid birthday value")
	}
	if !validDayMonth(day, month) {
		return 0, 0, nil, fmt.Errorf("invalid birthday date")
	}

	var yearPtr *int
	if strings.TrimSpace(yearRaw) != "" {
		year, err := strconv.Atoi(strings.TrimSpace(yearRaw))
		if err != nil {
			return 0, 0, nil, fmt.Errorf("invalid birthday year")
		}
		if year < 1900 || year > 3000 {
			return 0, 0, nil, fmt.Errorf("invalid birthday year")
		}
		yearPtr = &year
	}

	return day, month, yearPtr, nil
}

func validDayMonth(day, month int) bool {
	refYear := 2024
	t := time.Date(refYear, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return int(t.Month()) == month && t.Day() == day
}

func matchOrEmpty(m []string, idx int) string {
	if idx >= len(m) {
		return ""
	}
	return m[idx]
}

func fallbackString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func buildProfileInputHelpMessage(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason != "" {
		reason = "I couldn't save that yet (" + reason + "). "
	}

	return reason + "Reply with one or both lines in this format:\n```text\nmarch 25\njanuary 23, 2024\n```\nUse `month day` for birthday and `month day, year` for hire date (year is required)."
}

func buildSaveAckMessage(parsed parsedProfileInput) string {
	if parsed.HasBirthday && parsed.HasHireDate {
		return "Saved your birthday and hire date! Thank you for sharing with SlackCheers :yellow_heart::tada: We can't wait to celebrate you on your special day :birthday::partying_face: and your work anniversary!"
	}
	if parsed.HasBirthday {
		return "Saved your birthday! Thank you for sharing with SlackCheers :yellow_heart::tada: We can't wait to celebrate you on your special day :birthday::partying_face:"
	}
	if parsed.HasHireDate {
		return "Saved your hire date! Thank you for sharing with SlackCheers :yellow_heart::tada: We can't wait to celebrate your work anniversary!"
	}

	return "Saved your profile updates."
}
