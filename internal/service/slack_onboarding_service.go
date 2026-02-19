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
	slackUsersListURL         = "https://slack.com/api/users.list"
	slackConversationsOpenURL = "https://slack.com/api/conversations.open"
	slackChatPostMessageURL   = "https://slack.com/api/chat.postMessage"
)

type SlackOnboardingService struct {
	workspaceRepo  *repository.WorkspaceRepository
	onboardingRepo *repository.OnboardingRepository
	httpClient     *http.Client
}

type OnboardingDispatchResult struct {
	TotalMembers  int               `json:"total_members"`
	Sent          int               `json:"sent"`
	Skipped       int               `json:"skipped"`
	Failed        int               `json:"failed"`
	FailedUsers   []string          `json:"failed_users"`
	FailedDetails map[string]string `json:"failed_details"`
}

type slackUsersListResponse struct {
	OK       bool   `json:"ok"`
	Error    string `json:"error"`
	Needed   string `json:"needed"`
	Provided string `json:"provided"`
	Members  []struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Deleted   bool   `json:"deleted"`
		IsBot     bool   `json:"is_bot"`
		IsAppUser bool   `json:"is_app_user"`
		Profile   struct {
			DisplayName string `json:"display_name"`
			RealName    string `json:"real_name"`
			Image192    string `json:"image_192"`
		} `json:"profile"`
	} `json:"members"`
	ResponseMetadata struct {
		NextCursor string `json:"next_cursor"`
	} `json:"response_metadata"`
}

type slackConversationsOpenResponse struct {
	OK       bool   `json:"ok"`
	Error    string `json:"error"`
	Needed   string `json:"needed"`
	Provided string `json:"provided"`
	Channel  struct {
		ID string `json:"id"`
	} `json:"channel"`
}

type slackPostMessageResponse struct {
	OK       bool   `json:"ok"`
	Error    string `json:"error"`
	Needed   string `json:"needed"`
	Provided string `json:"provided"`
}

type slackMember struct {
	ID          string
	DisplayName string
}

func NewSlackOnboardingService(workspaceRepo *repository.WorkspaceRepository, onboardingRepo *repository.OnboardingRepository) *SlackOnboardingService {
	return &SlackOnboardingService{
		workspaceRepo:  workspaceRepo,
		onboardingRepo: onboardingRepo,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (s *SlackOnboardingService) SendOnboardingDMs(ctx context.Context, workspaceID string, force bool) (OnboardingDispatchResult, error) {
	install, err := s.workspaceRepo.GetSlackInstallationByWorkspaceID(ctx, workspaceID)
	if err != nil {
		return OnboardingDispatchResult{}, err
	}
	if strings.TrimSpace(install.BotToken) == "" {
		return OnboardingDispatchResult{}, fmt.Errorf("workspace is not connected to Slack yet")
	}

	members, err := s.listWorkspaceMembers(ctx, install.BotToken)
	if err != nil {
		return OnboardingDispatchResult{}, err
	}

	sentUsers := map[string]struct{}{}
	if !force {
		sentUsers, err = s.onboardingRepo.ListSentUserIDs(ctx, workspaceID)
		if err != nil {
			return OnboardingDispatchResult{}, err
		}
	}

	result := OnboardingDispatchResult{
		TotalMembers:  len(members),
		FailedUsers:   make([]string, 0),
		FailedDetails: make(map[string]string),
	}

	for _, member := range members {
		if _, alreadySent := sentUsers[member.ID]; alreadySent {
			result.Skipped++
			continue
		}

		message := buildOnboardingMessage(member.DisplayName)
		if err := s.sendDirectMessage(ctx, install.BotToken, member.ID, message); err != nil {
			result.Failed++
			result.FailedUsers = append(result.FailedUsers, member.ID)
			result.FailedDetails[member.ID] = err.Error()
			continue
		}

		if err := s.onboardingRepo.MarkSent(ctx, workspaceID, member.ID); err != nil {
			result.Failed++
			result.FailedUsers = append(result.FailedUsers, member.ID)
			result.FailedDetails[member.ID] = err.Error()
			continue
		}

		result.Sent++
	}

	sort.Strings(result.FailedUsers)
	return result, nil
}

func (s *SlackOnboardingService) listWorkspaceMembers(ctx context.Context, botToken string) ([]slackMember, error) {
	members := make([]slackMember, 0)
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

func (s *SlackOnboardingService) listUsersPage(ctx context.Context, botToken, cursor string) ([]slackMember, string, error) {
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

	members := make([]slackMember, 0, len(payload.Members))
	for _, m := range payload.Members {
		if m.ID == "" || m.Deleted || m.IsBot || m.IsAppUser || m.ID == "USLACKBOT" || strings.EqualFold(strings.TrimSpace(m.Name), "slackbot") {
			continue
		}
		name := strings.TrimSpace(m.Profile.DisplayName)
		if name == "" {
			name = strings.TrimSpace(m.Profile.RealName)
		}
		if name == "" {
			name = strings.TrimSpace(m.Name)
		}
		members = append(members, slackMember{ID: m.ID, DisplayName: name})
	}

	return members, payload.ResponseMetadata.NextCursor, nil
}

func (s *SlackOnboardingService) sendDirectMessage(ctx context.Context, botToken, userID, text string) error {
	channelID, err := s.openDMChannel(ctx, botToken, userID)
	if err != nil {
		return err
	}

	payload := map[string]any{
		"channel": channelID,
		"text":    text,
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, slackChatPostMessageURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build chat.postMessage request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+botToken)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call chat.postMessage: %w", err)
	}
	defer resp.Body.Close()

	var parsed slackPostMessageResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return fmt.Errorf("decode chat.postMessage response: %w", err)
	}
	if !parsed.OK {
		if parsed.Error == "" {
			parsed.Error = "chat.postMessage failed"
		}
		return fmt.Errorf("slack api error: %s%s", parsed.Error, slackScopeHint(parsed.Needed, parsed.Provided))
	}

	return nil
}

func (s *SlackOnboardingService) openDMChannel(ctx context.Context, botToken, userID string) (string, error) {
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
	if strings.TrimSpace(parsed.Channel.ID) == "" {
		return "", fmt.Errorf("slack api error: missing dm channel id")
	}

	return parsed.Channel.ID, nil
}

func buildOnboardingMessage(name string) string {
	cleanName := strings.TrimSpace(name)
	cleanName = strings.TrimRight(cleanName, ".!?,")
	if cleanName == "" {
		cleanName = "there"
	}

	return fmt.Sprintf(
		"Hi %s!\n\nSlackCheers is now active in your workspace to celebrate great moments.\n\nTell us your birthday: `month day` and hire date: `month day, year`\n\nYou can send only birthday or only hire date, and update later anytime.",
		cleanName,
	)
}
