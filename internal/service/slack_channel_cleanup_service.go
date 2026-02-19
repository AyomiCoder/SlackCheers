package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"slackcheers/internal/repository"
)

type SlackChannelCleanupService struct {
	workspaceRepo *repository.WorkspaceRepository
	httpClient    *http.Client
}

type ChannelCleanupResult struct {
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

func NewSlackChannelCleanupService(workspaceRepo *repository.WorkspaceRepository) *SlackChannelCleanupService {
	return &SlackChannelCleanupService{
		workspaceRepo: workspaceRepo,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (s *SlackChannelCleanupService) CleanupBirthdayMessages(
	ctx context.Context,
	workspaceID, channelID, match string,
) (ChannelCleanupResult, error) {
	channelID = strings.TrimSpace(channelID)
	if channelID == "" {
		return ChannelCleanupResult{}, fmt.Errorf("channel_id is required")
	}

	match = strings.TrimSpace(match)
	if match == "" {
		match = "happy birthday"
	}

	install, err := s.workspaceRepo.GetSlackInstallationByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return ChannelCleanupResult{}, err
	}
	if strings.TrimSpace(install.BotToken) == "" {
		return ChannelCleanupResult{}, fmt.Errorf("workspace is not connected to Slack yet")
	}

	slackChannelID, err := s.resolveSlackChannelID(ctx, workspaceID, channelID)
	if err != nil {
		return ChannelCleanupResult{}, err
	}

	messages, err := s.listChannelHistory(ctx, install.BotToken, slackChannelID)
	if err != nil {
		return ChannelCleanupResult{}, err
	}

	result := ChannelCleanupResult{
		ChannelID:      channelID,
		SlackChannelID: slackChannelID,
		Match:          match,
		Scanned:        len(messages),
		FailedTS:       make([]string, 0),
		FailedDetails:  make(map[string]string),
	}

	for _, msg := range messages {
		if !isBotAuthoredDMMessage(msg, install.BotUserID) {
			continue
		}
		if !strings.Contains(strings.ToLower(msg.Text), strings.ToLower(match)) {
			continue
		}

		result.Matched++
		if err := s.deleteMessage(ctx, install.BotToken, slackChannelID, msg.TS); err != nil {
			result.Failed++
			result.FailedTS = append(result.FailedTS, msg.TS)
			result.FailedDetails[msg.TS] = err.Error()
			continue
		}
		result.Deleted++
	}

	sort.Strings(result.FailedTS)
	return result, nil
}

func (s *SlackChannelCleanupService) resolveSlackChannelID(ctx context.Context, workspaceID, channelID string) (string, error) {
	channels, err := s.workspaceRepo.ListChannelsByWorkspace(ctx, workspaceID)
	if err != nil {
		return "", err
	}

	for _, ch := range channels {
		if ch.ID == channelID || ch.SlackChannelID == channelID {
			return ch.SlackChannelID, nil
		}
	}

	// If no configured channel match is found, assume caller passed a raw Slack channel ID.
	return channelID, nil
}

func (s *SlackChannelCleanupService) listChannelHistory(ctx context.Context, botToken, channelID string) ([]slackDMMessage, error) {
	result := make([]slackDMMessage, 0)
	cursor := ""

	for page := 0; page < 20; page++ {
		pageMessages, nextCursor, err := s.listChannelHistoryPage(ctx, botToken, channelID, cursor)
		if err != nil {
			return nil, err
		}
		result = append(result, pageMessages...)

		if strings.TrimSpace(nextCursor) == "" {
			break
		}
		cursor = nextCursor
	}

	return result, nil
}

func (s *SlackChannelCleanupService) listChannelHistoryPage(ctx context.Context, botToken, channelID, cursor string) ([]slackDMMessage, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, slackConversationsHistoryURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("build conversations.history request: %w", err)
	}

	q := req.URL.Query()
	q.Set("channel", channelID)
	q.Set("limit", "200")
	if strings.TrimSpace(cursor) != "" {
		q.Set("cursor", cursor)
	}
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Authorization", "Bearer "+botToken)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("call conversations.history: %w", err)
	}
	defer resp.Body.Close()

	var parsed slackConversationsHistoryResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, "", fmt.Errorf("decode conversations.history response: %w", err)
	}
	if !parsed.OK {
		if parsed.Error == "" {
			parsed.Error = "conversations.history failed"
		}
		return nil, "", fmt.Errorf("slack api error: %s%s", parsed.Error, slackScopeHint(parsed.Needed, parsed.Provided))
	}

	messages := make([]slackDMMessage, 0, len(parsed.Messages))
	for _, m := range parsed.Messages {
		messages = append(messages, slackDMMessage{
			TS:      strings.TrimSpace(m.TS),
			User:    strings.TrimSpace(m.User),
			BotID:   strings.TrimSpace(m.BotID),
			Subtype: strings.TrimSpace(m.Subtype),
			Text:    m.Text,
		})
	}

	return messages, strings.TrimSpace(parsed.ResponseMetadata.NextCursor), nil
}

func (s *SlackChannelCleanupService) deleteMessage(ctx context.Context, botToken, channelID, ts string) error {
	payload := map[string]any{
		"channel": channelID,
		"ts":      ts,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, slackChatDeleteURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build chat.delete request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+botToken)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call chat.delete: %w", err)
	}
	defer resp.Body.Close()

	var parsed slackChatDeleteResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return fmt.Errorf("decode chat.delete response: %w", err)
	}
	if !parsed.OK {
		if parsed.Error == "" {
			parsed.Error = "chat.delete failed"
		}
		return fmt.Errorf("slack api error: %s%s", parsed.Error, slackScopeHint(parsed.Needed, parsed.Provided))
	}

	return nil
}
