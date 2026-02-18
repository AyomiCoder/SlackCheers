package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"slackcheers/internal/repository"
)

const (
	slackChatPostMessageURL   = "https://slack.com/api/chat.postMessage"
	slackConversationsOpenURL = "https://slack.com/api/conversations.open"
)

type APIClient struct {
	workspaceRepo   *repository.WorkspaceRepository
	defaultBotToken string
	logger          *slog.Logger
	httpClient      *http.Client
}

type slackAPIResponse struct {
	OK       bool   `json:"ok"`
	Error    string `json:"error"`
	Needed   string `json:"needed"`
	Provided string `json:"provided"`
	Channel  struct {
		ID string `json:"id"`
	} `json:"channel"`
}

func NewClient(workspaceRepo *repository.WorkspaceRepository, defaultBotToken string, logger *slog.Logger) (Client, error) {
	if workspaceRepo == nil {
		return nil, fmt.Errorf("workspace repository is required")
	}

	return &APIClient{
		workspaceRepo:   workspaceRepo,
		defaultBotToken: strings.TrimSpace(defaultBotToken),
		logger:          logger,
		httpClient: &http.Client{
			Timeout: 12 * time.Second,
		},
	}, nil
}

func (c *APIClient) PostMessage(ctx context.Context, workspaceID, channelID, text string, avatarURLs []string) error {
	token, err := c.resolveBotToken(ctx, workspaceID)
	if err != nil {
		return err
	}

	payload := map[string]any{
		"channel": channelID,
		"text":    text,
	}

	if len(avatarURLs) > 0 {
		blocks := make([]map[string]any, 0, 1+len(avatarURLs))
		blocks = append(blocks, map[string]any{
			"type": "section",
			"text": map[string]any{
				"type": "mrkdwn",
				"text": text,
			},
		})

		for i, avatar := range avatarURLs {
			if i >= 8 {
				break
			}
			avatar = strings.TrimSpace(avatar)
			if avatar == "" {
				continue
			}
			blocks = append(blocks, map[string]any{
				"type":      "image",
				"image_url": avatar,
				"alt_text":  "celebrant_avatar",
			})
		}

		if len(blocks) > 1 {
			payload["blocks"] = blocks
		}
	}

	if err := c.callSlackJSON(ctx, token, slackChatPostMessageURL, payload, nil); err != nil {
		c.logger.ErrorContext(ctx, "slack post message failed", slog.String("workspace_id", workspaceID), slog.String("channel_id", channelID), slog.String("error", err.Error()))
		return err
	}

	return nil
}

func (c *APIClient) SendDirectMessage(ctx context.Context, workspaceID, userID, text string) error {
	token, err := c.resolveBotToken(ctx, workspaceID)
	if err != nil {
		return err
	}

	dmResp := slackAPIResponse{}
	if err := c.callSlackJSON(ctx, token, slackConversationsOpenURL, map[string]any{"users": userID}, &dmResp); err != nil {
		return err
	}

	channelID := strings.TrimSpace(dmResp.Channel.ID)
	if channelID == "" {
		return fmt.Errorf("slack api error: missing dm channel id")
	}

	if err := c.callSlackJSON(ctx, token, slackChatPostMessageURL, map[string]any{
		"channel": channelID,
		"text":    text,
	}, nil); err != nil {
		return err
	}

	return nil
}

func (c *APIClient) resolveBotToken(ctx context.Context, workspaceID string) (string, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID != "" {
		install, err := c.workspaceRepo.GetSlackInstallationByWorkspaceID(ctx, workspaceID)
		if err != nil {
			if !errors.Is(err, repository.ErrNotFound) {
				return "", fmt.Errorf("resolve workspace bot token: %w", err)
			}
		} else {
			token := strings.TrimSpace(install.BotToken)
			if token != "" {
				return token, nil
			}
		}
	}

	if c.defaultBotToken != "" {
		return c.defaultBotToken, nil
	}

	return "", fmt.Errorf("no Slack bot token configured for workspace %q", workspaceID)
}

func (c *APIClient) callSlackJSON(ctx context.Context, token, endpoint string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal slack payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build slack request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("call slack api: %w", err)
	}
	defer resp.Body.Close()

	var parsed slackAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return fmt.Errorf("decode slack response: %w", err)
	}

	if !parsed.OK {
		if parsed.Error == "" {
			parsed.Error = "unknown_error"
		}
		return fmt.Errorf("slack api error: %s%s", parsed.Error, slackScopeHint(parsed.Needed, parsed.Provided))
	}

	if out != nil {
		b, _ := json.Marshal(parsed)
		if err := json.Unmarshal(b, out); err != nil {
			return fmt.Errorf("decode slack output: %w", err)
		}
	}

	return nil
}

func ValidatePlaceholders(template string) error {
	if template == "" {
		return fmt.Errorf("template cannot be empty")
	}
	return nil
}

func slackScopeHint(needed, provided string) string {
	needed = strings.TrimSpace(needed)
	provided = strings.TrimSpace(provided)
	if needed == "" && provided == "" {
		return ""
	}
	if provided == "" {
		return fmt.Sprintf(" (needed=%s)", needed)
	}
	if needed == "" {
		return fmt.Sprintf(" (provided=%s)", provided)
	}
	return fmt.Sprintf(" (needed=%s provided=%s)", needed, provided)
}
