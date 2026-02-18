package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"slackcheers/internal/domain"
	"slackcheers/internal/repository"
	"slackcheers/internal/slack"
)

type CelebrationService struct {
	workspaceRepo *repository.WorkspaceRepository
	peopleRepo    *repository.PeopleRepository
	slackClient   slack.Client
	logger        *slog.Logger
}

func NewCelebrationService(
	workspaceRepo *repository.WorkspaceRepository,
	peopleRepo *repository.PeopleRepository,
	slackClient slack.Client,
	logger *slog.Logger,
) *CelebrationService {
	return &CelebrationService{
		workspaceRepo: workspaceRepo,
		peopleRepo:    peopleRepo,
		slackClient:   slackClient,
		logger:        logger,
	}
}

func (s *CelebrationService) RunDueCelebrations(ctx context.Context, now time.Time) error {
	channels, err := s.workspaceRepo.ListDueChannels(ctx, now)
	if err != nil {
		return err
	}

	for _, channel := range channels {
		if err := s.runChannelCelebration(ctx, channel, now); err != nil {
			s.logger.ErrorContext(ctx, "failed channel celebration run",
				slog.String("channel_id", channel.ID),
				slog.String("workspace_id", channel.WorkspaceID),
				slog.String("error", err.Error()),
			)
			continue
		}
	}

	return nil
}

func (s *CelebrationService) runChannelCelebration(ctx context.Context, channel domain.WorkspaceChannel, now time.Time) error {
	loc, err := time.LoadLocation(channel.Timezone)
	if err != nil {
		return fmt.Errorf("invalid channel timezone %q: %w", channel.Timezone, err)
	}

	localNow := now.In(loc)
	month := int(localNow.Month())
	day := localNow.Day()
	year := localNow.Year()

	if channel.BirthdaysEnabled {
		birthdays, err := s.peopleRepo.FindBirthdaysByWorkspaceAndDate(ctx, channel.WorkspaceID, month, day)
		if err != nil {
			return err
		}
		if len(birthdays) > 0 {
			message := renderTemplate(channel.BirthdayTemplate, birthdays, nil)
			message = appendBrandingEmoji(message, channel.BrandingEmoji)

			if err := s.slackClient.PostMessage(ctx, channel.SlackChannelID, message, avatarURLs(birthdays)); err != nil {
				return fmt.Errorf("post birthday message: %w", err)
			}
		}
	}

	if channel.AnniversariesEnabled {
		anniversaries, err := s.peopleRepo.FindAnniversariesByWorkspaceAndDate(ctx, channel.WorkspaceID, month, day, year)
		if err != nil {
			return err
		}
		if len(anniversaries) > 0 {
			message := renderAnniversaryTemplate(channel.AnniversaryTemplate, anniversaries)
			message = appendBrandingEmoji(message, channel.BrandingEmoji)

			if err := s.slackClient.PostMessage(ctx, channel.SlackChannelID, message, avatarURLsFromAnniversaries(anniversaries)); err != nil {
				return fmt.Errorf("post anniversary message: %w", err)
			}
		}
	}

	if err := s.workspaceRepo.MarkChannelDispatched(ctx, channel.ID, localNow); err != nil {
		return err
	}

	return nil
}

func renderTemplate(template string, people []domain.Person, _ []domain.AnniversaryPerson) string {
	users := mentionPeople(people)
	msg := strings.ReplaceAll(template, "{users}", users)
	msg = strings.ReplaceAll(msg, "{years}", "")
	return strings.TrimSpace(msg)
}

func renderAnniversaryTemplate(template string, anniversaries []domain.AnniversaryPerson) string {
	mentions := make([]string, 0, len(anniversaries))
	years := make([]string, 0, len(anniversaries))
	for _, a := range anniversaries {
		mentions = append(mentions, fmt.Sprintf("<@%s>", a.SlackUserID))
		years = append(years, fmt.Sprintf("%d", a.Years))
	}
	msg := strings.ReplaceAll(template, "{users}", strings.Join(mentions, ", "))
	msg = strings.ReplaceAll(msg, "{years}", strings.Join(years, ", "))
	return strings.TrimSpace(msg)
}

func mentionPeople(people []domain.Person) string {
	mentions := make([]string, 0, len(people))
	for _, p := range people {
		mentions = append(mentions, fmt.Sprintf("<@%s>", p.SlackUserID))
	}
	return strings.Join(mentions, ", ")
}

func avatarURLs(people []domain.Person) []string {
	urls := make([]string, 0, len(people))
	for _, p := range people {
		if p.AvatarURL != "" {
			urls = append(urls, p.AvatarURL)
		}
	}
	return urls
}

func avatarURLsFromAnniversaries(people []domain.AnniversaryPerson) []string {
	urls := make([]string, 0, len(people))
	for _, p := range people {
		if p.AvatarURL != "" {
			urls = append(urls, p.AvatarURL)
		}
	}
	return urls
}

func appendBrandingEmoji(message, emoji string) string {
	emoji = strings.TrimSpace(emoji)
	if emoji == "" {
		return message
	}
	return message + " " + emoji
}
