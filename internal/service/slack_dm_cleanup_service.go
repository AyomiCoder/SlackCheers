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

const (
	slackConversationsHistoryURL = "https://slack.com/api/conversations.history"
	slackChatDeleteURL           = "https://slack.com/api/chat.delete"
)

type SlackDMCleanupService struct {
	workspaceRepo *repository.WorkspaceRepository
	httpClient    *http.Client
}

type DMCleanupResult struct {
	UserID        string            `json:"user_id"`
	ChannelID     string            `json:"channel_id"`
	TotalMessages int               `json:"total_messages"`
	BotMessages   int               `json:"bot_messages"`
	Deleted       int               `json:"deleted"`
	Failed        int               `json:"failed"`
	FailedTS      []string          `json:"failed_ts"`
	FailedDetails map[string]string `json:"failed_details"`
}

type slackConversationsHistoryResponse struct {
	OK       bool   `json:"ok"`
	Error    string `json:"error"`
	Needed   string `json:"needed"`
	Provided string `json:"provided"`
	Messages []struct {
		TS      string `json:"ts"`
		User    string `json:"user"`
		BotID   string `json:"bot_id"`
		Subtype string `json:"subtype"`
		Text    string `json:"text"`
	} `json:"messages"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

type slackChatDeleteResponse struct {
	OK       bool   `json:"ok"`
	Error    string `json:"error"`
	Needed   string `json:"needed"`
	Provided string `json:"provided"`
}

type slackDMMessage struct {
	TS      string
	User    string
	BotID   string
	Subtype string
	Text    string
}

func NewSlackDMCleanupService(workspaceRepo *repository.WorkspaceRepository) *SlackDMCleanupService {
	return &SlackDMCleanupService{
		workspaceRepo: workspaceRepo,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (s *SlackDMCleanupService) CleanupBotDirectMessages(ctx context.Context, workspaceID, userID string) (DMCleanupResult, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return DMCleanupResult{}, fmt.Errorf("user_id is required")
	}

	install, err := s.workspaceRepo.GetSlackInstallationByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return DMCleanupResult{}, err
	}
	if strings.TrimSpace(install.BotToken) == "" {
		return DMCleanupResult{}, fmt.Errorf("workspace is not connected to Slack yet")
	}

	channelID, err := s.openDMChannel(ctx, install.BotToken, userID)
	if err != nil {
		return DMCleanupResult{}, err
	}

	messages, err := s.listDMHistory(ctx, install.BotToken, channelID)
	if err != nil {
		return DMCleanupResult{}, err
	}

	result := DMCleanupResult{
		UserID:        userID,
		ChannelID:     channelID,
		TotalMessages: len(messages),
		FailedTS:      make([]string, 0),
		FailedDetails: make(map[string]string),
	}

	for _, msg := range messages {
		if !isBotAuthoredDMMessage(msg, install.BotUserID) {
			continue
		}
		result.BotMessages++

		if err := s.deleteDMMessage(ctx, install.BotToken, channelID, msg.TS); err != nil {
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

func (s *SlackDMCleanupService) openDMChannel(ctx context.Context, botToken, userID string) (string, error) {
	payload := map[string]any{"users": userID}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, slackConversationsOpenURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build conversations.open request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+botToken)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call conversations.open: %w", err)
	}
	defer resp.Body.Close()

	var parsed slackConversationsOpenResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", fmt.Errorf("decode conversations.open response: %w", err)
	}
	if !parsed.OK {
		if parsed.Error == "" {
			parsed.Error = "conversations.open failed"
		}
		return "", fmt.Errorf("slack api error: %s%s", parsed.Error, slackScopeHint(parsed.Needed, parsed.Provided))
	}

	channelID := strings.TrimSpace(parsed.Channel.ID)
	if channelID == "" {
		return "", fmt.Errorf("slack api error: missing dm channel id")
	}

	return channelID, nil
}

func (s *SlackDMCleanupService) listDMHistory(ctx context.Context, botToken, channelID string) ([]slackDMMessage, error) {
	result := make([]slackDMMessage, 0)
	cursor := ""

	for page := 0; page < 20; page++ {
		pageMessages, nextCursor, err := s.listDMHistoryPage(ctx, botToken, channelID, cursor)
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

func (s *SlackDMCleanupService) listDMHistoryPage(ctx context.Context, botToken, channelID, cursor string) ([]slackDMMessage, string, error) {
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

func (s *SlackDMCleanupService) deleteDMMessage(ctx context.Context, botToken, channelID, ts string) error {
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

func isBotAuthoredDMMessage(msg slackDMMessage, botUserID string) bool {
	if strings.TrimSpace(msg.TS) == "" {
		return false
	}
	if strings.TrimSpace(msg.BotID) != "" {
		return true
	}
	if strings.TrimSpace(botUserID) != "" && strings.TrimSpace(msg.User) == strings.TrimSpace(botUserID) {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(msg.Subtype), "bot_message")
}
