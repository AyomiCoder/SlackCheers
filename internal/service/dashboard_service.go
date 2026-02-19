package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"slackcheers/internal/domain"
	"slackcheers/internal/repository"
)

type DashboardService struct {
	workspaceRepo *repository.WorkspaceRepository
	peopleRepo    *repository.PeopleRepository
	httpClient    *http.Client
}

func NewDashboardService(workspaceRepo *repository.WorkspaceRepository, peopleRepo *repository.PeopleRepository) *DashboardService {
	return &DashboardService{
		workspaceRepo: workspaceRepo,
		peopleRepo:    peopleRepo,
		httpClient: &http.Client{
			Timeout: 12 * time.Second,
		},
	}
}

func (s *DashboardService) ListPeople(ctx context.Context, workspaceID string) ([]domain.Person, error) {
	existing, err := s.peopleRepo.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	install, err := s.workspaceRepo.GetSlackInstallationByWorkspaceID(ctx, workspaceID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, repository.ErrNotFound
		}
		return nil, err
	}
	if strings.TrimSpace(install.BotToken) == "" {
		return existing, nil
	}

	members, err := s.listWorkspaceMembers(ctx, install.BotToken)
	if err != nil {
		return nil, err
	}

	return mergePeopleWithWorkspaceMembers(existing, members, workspaceID), nil
}

func (s *DashboardService) UpsertPerson(ctx context.Context, in repository.UpsertPersonInput) (domain.Person, error) {
	if in.RemindersMode == "" {
		in.RemindersMode = "same_day"
	}
	return s.peopleRepo.Upsert(ctx, in)
}

func (s *DashboardService) ListChannels(ctx context.Context, workspaceID string) ([]domain.WorkspaceChannel, error) {
	return s.workspaceRepo.ListChannelsByWorkspace(ctx, workspaceID)
}

func (s *DashboardService) UpdateChannelSettings(
	ctx context.Context,
	workspaceID, channelID, postingTime, timezone string,
	birthdaysEnabled, anniversariesEnabled bool,
) (domain.WorkspaceChannel, error) {
	if _, err := time.Parse("15:04", postingTime); err != nil {
		return domain.WorkspaceChannel{}, fmt.Errorf("posting time must use HH:MM format")
	}

	if _, err := time.LoadLocation(timezone); err != nil {
		return domain.WorkspaceChannel{}, fmt.Errorf("invalid timezone")
	}

	return s.workspaceRepo.UpdateChannelSettings(
		ctx,
		workspaceID,
		channelID,
		postingTime,
		timezone,
		birthdaysEnabled,
		anniversariesEnabled,
	)
}

func (s *DashboardService) UpdateChannelTemplates(
	ctx context.Context,
	workspaceID, channelID, birthdayTemplate, anniversaryTemplate, brandingEmoji string,
) (domain.WorkspaceChannel, error) {
	if birthdayTemplate == "" || anniversaryTemplate == "" {
		return domain.WorkspaceChannel{}, fmt.Errorf("templates cannot be empty")
	}

	return s.workspaceRepo.UpdateChannelTemplates(ctx, workspaceID, channelID, birthdayTemplate, anniversaryTemplate, brandingEmoji)
}

func (s *DashboardService) Overview(ctx context.Context, workspaceID string, days int, celebrationType string) ([]domain.UpcomingCelebration, error) {
	if days <= 0 {
		days = 30
	}

	people, err := s.peopleRepo.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC().Truncate(24 * time.Hour)
	end := now.AddDate(0, 0, days)

	items := make([]domain.UpcomingCelebration, 0)
	for _, p := range people {
		if celebrationType == "all" || celebrationType == "birthdays" {
			if p.BirthdayMonth != nil && p.BirthdayDay != nil {
				nextBirthday := nextOccurrence(now, *p.BirthdayMonth, *p.BirthdayDay)
				if !nextBirthday.After(end) {
					items = append(items, domain.UpcomingCelebration{
						Date:      nextBirthday,
						Type:      "birthday",
						UserID:    p.SlackUserID,
						SlackUser: p.SlackHandle,
						Name:      p.DisplayName,
					})
				}
			}
		}

		if celebrationType == "all" || celebrationType == "anniversaries" {
			if p.HireDate != nil {
				nextAnniversary := nextOccurrence(now, int(p.HireDate.Month()), p.HireDate.Day())
				if !nextAnniversary.After(end) {
					years := nextAnniversary.Year() - p.HireDate.Year()
					items = append(items, domain.UpcomingCelebration{
						Date:      nextAnniversary,
						Type:      "anniversary",
						UserID:    p.SlackUserID,
						SlackUser: p.SlackHandle,
						Name:      p.DisplayName,
						Years:     &years,
					})
				}
			}
		}
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Date.Equal(items[j].Date) {
			return items[i].Name < items[j].Name
		}
		return items[i].Date.Before(items[j].Date)
	})

	return items, nil
}

func nextOccurrence(from time.Time, month, day int) time.Time {
	candidate := time.Date(from.Year(), time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if candidate.Before(from) {
		candidate = candidate.AddDate(1, 0, 0)
	}
	return candidate
}

type dashboardWorkspaceMember struct {
	ID          string
	Handle      string
	DisplayName string
	AvatarURL   string
}

func (s *DashboardService) listWorkspaceMembers(ctx context.Context, botToken string) ([]dashboardWorkspaceMember, error) {
	members := make([]dashboardWorkspaceMember, 0)
	cursor := ""

	for page := 0; page < 10; page++ {
		pageMembers, nextCursor, err := s.listUsersPage(ctx, botToken, cursor)
		if err != nil {
			return nil, err
		}
		members = append(members, pageMembers...)
		if strings.TrimSpace(nextCursor) == "" {
			break
		}
		cursor = nextCursor
	}

	return members, nil
}

func (s *DashboardService) listUsersPage(ctx context.Context, botToken, cursor string) ([]dashboardWorkspaceMember, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, slackUsersListURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("build users.list request: %w", err)
	}

	q := req.URL.Query()
	q.Set("limit", "200")
	if strings.TrimSpace(cursor) != "" {
		q.Set("cursor", cursor)
	}
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Authorization", "Bearer "+botToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("call users.list: %w", err)
	}
	defer resp.Body.Close()

	var payload slackUsersListResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, "", fmt.Errorf("decode users.list response: %w", err)
	}
	if !payload.OK {
		if payload.Error == "" {
			payload.Error = "users.list failed"
		}
		return nil, "", fmt.Errorf("slack api error: %s%s", payload.Error, slackScopeHint(payload.Needed, payload.Provided))
	}

	members := make([]dashboardWorkspaceMember, 0, len(payload.Members))
	for _, m := range payload.Members {
		if m.ID == "" || m.Deleted || m.IsBot || m.IsAppUser || m.ID == "USLACKBOT" || strings.EqualFold(strings.TrimSpace(m.Name), "slackbot") {
			continue
		}

		displayName := strings.TrimSpace(m.Profile.DisplayName)
		if displayName == "" {
			displayName = strings.TrimSpace(m.Profile.RealName)
		}
		if displayName == "" {
			displayName = strings.TrimSpace(m.Name)
		}

		members = append(members, dashboardWorkspaceMember{
			ID:          strings.TrimSpace(m.ID),
			Handle:      strings.TrimSpace(m.Name),
			DisplayName: displayName,
			AvatarURL:   strings.TrimSpace(m.Profile.Image192),
		})
	}

	return members, payload.ResponseMetadata.NextCursor, nil
}

func mergePeopleWithWorkspaceMembers(existing []domain.Person, members []dashboardWorkspaceMember, workspaceID string) []domain.Person {
	byUserID := make(map[string]domain.Person, len(existing))
	for _, p := range existing {
		byUserID[p.SlackUserID] = p
	}

	merged := make([]domain.Person, 0, len(existing)+len(members))
	for _, m := range members {
		if p, ok := byUserID[m.ID]; ok {
			if strings.TrimSpace(p.SlackHandle) == "" {
				p.SlackHandle = m.Handle
			}
			if strings.TrimSpace(p.DisplayName) == "" {
				p.DisplayName = m.DisplayName
			}
			if strings.TrimSpace(p.AvatarURL) == "" {
				p.AvatarURL = m.AvatarURL
			}
			if p.WorkspaceID == "" {
				p.WorkspaceID = workspaceID
			}
			if strings.TrimSpace(p.RemindersMode) == "" {
				p.RemindersMode = "same_day"
			}
			merged = append(merged, p)
			delete(byUserID, m.ID)
			continue
		}

		merged = append(merged, domain.Person{
			WorkspaceID:            workspaceID,
			SlackUserID:            m.ID,
			SlackHandle:            m.Handle,
			DisplayName:            m.DisplayName,
			AvatarURL:              m.AvatarURL,
			PublicCelebrationOptIn: true,
			RemindersMode:          "same_day",
		})
	}

	for _, p := range byUserID {
		if strings.TrimSpace(p.RemindersMode) == "" {
			p.RemindersMode = "same_day"
		}
		merged = append(merged, p)
	}

	sort.Slice(merged, func(i, j int) bool {
		left := strings.ToLower(strings.TrimSpace(fallbackString(merged[i].DisplayName, merged[i].SlackHandle, merged[i].SlackUserID)))
		right := strings.ToLower(strings.TrimSpace(fallbackString(merged[j].DisplayName, merged[j].SlackHandle, merged[j].SlackUserID)))
		if left == right {
			return strings.ToLower(merged[i].SlackUserID) < strings.ToLower(merged[j].SlackUserID)
		}
		return left < right
	})

	return merged
}
