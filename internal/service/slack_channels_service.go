package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"slackcheers/internal/repository"
)

const slackConversationsListURL = "https://slack.com/api/conversations.list"

type SlackChannelsService struct {
	workspaceRepo *repository.WorkspaceRepository
	httpClient    *http.Client
}

type SlackChannel struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsPrivate bool   `json:"is_private"`
}

type slackConversationsListResponse struct {
	OK       bool   `json:"ok"`
	Error    string `json:"error"`
	Channels []struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		IsPrivate  bool   `json:"is_private"`
		IsArchived bool   `json:"is_archived"`
	} `json:"channels"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

func NewSlackChannelsService(workspaceRepo *repository.WorkspaceRepository) *SlackChannelsService {
	return &SlackChannelsService{
		workspaceRepo: workspaceRepo,
		httpClient: &http.Client{
			Timeout: 12 * time.Second,
		},
	}
}

func (s *SlackChannelsService) ListChannels(ctx context.Context, workspaceID string) ([]SlackChannel, error) {
	installation, err := s.workspaceRepo.GetSlackInstallationByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(installation.BotToken) == "" {
		return nil, fmt.Errorf("workspace is not connected to Slack yet")
	}

	channels := make([]SlackChannel, 0)
	cursor := ""
	for i := 0; i < 10; i++ {
		page, nextCursor, err := s.listChannelsPage(ctx, installation.BotToken, cursor)
		if err != nil {
			return nil, err
		}
		channels = append(channels, page...)

		if strings.TrimSpace(nextCursor) == "" {
			break
		}
		cursor = nextCursor
	}

	sort.Slice(channels, func(i, j int) bool {
		return strings.ToLower(channels[i].Name) < strings.ToLower(channels[j].Name)
	})

	return channels, nil
}

func (s *SlackChannelsService) listChannelsPage(ctx context.Context, botToken, cursor string) ([]SlackChannel, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, slackConversationsListURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("build slack conversations request: %w", err)
	}

	q := req.URL.Query()
	q.Set("types", "public_channel")
	q.Set("exclude_archived", "true")
	q.Set("limit", "200")
	if strings.TrimSpace(cursor) != "" {
		q.Set("cursor", cursor)
	}
	req.URL.RawQuery = q.Encode()

	req.Header.Set("Authorization", "Bearer "+botToken)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("call slack conversations.list: %w", err)
	}
	defer resp.Body.Close()

	var payload slackConversationsListResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, "", fmt.Errorf("decode slack conversations response: %w", err)
	}
	if !payload.OK {
		if payload.Error == "" {
			payload.Error = "conversations.list failed"
		}
		return nil, "", fmt.Errorf("slack api error: %s", payload.Error)
	}

	channels := make([]SlackChannel, 0, len(payload.Channels))
	for _, ch := range payload.Channels {
		if ch.ID == "" || ch.Name == "" || ch.IsArchived {
			continue
		}
		channels = append(channels, SlackChannel{
			ID:        ch.ID,
			Name:      ch.Name,
			IsPrivate: ch.IsPrivate,
		})
	}

	return channels, payload.ResponseMetadata.NextCursor, nil
}
