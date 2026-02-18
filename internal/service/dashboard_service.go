package service

import (
	"context"
	"fmt"
	"sort"
	"time"

	"slackcheers/internal/domain"
	"slackcheers/internal/repository"
)

type DashboardService struct {
	workspaceRepo *repository.WorkspaceRepository
	peopleRepo    *repository.PeopleRepository
}

func NewDashboardService(workspaceRepo *repository.WorkspaceRepository, peopleRepo *repository.PeopleRepository) *DashboardService {
	return &DashboardService{workspaceRepo: workspaceRepo, peopleRepo: peopleRepo}
}

func (s *DashboardService) ListPeople(ctx context.Context, workspaceID string) ([]domain.Person, error) {
	return s.peopleRepo.ListByWorkspace(ctx, workspaceID)
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
