package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"slackcheers/internal/config"
	"slackcheers/internal/repository"
)

const slackOAuthAccessURL = "https://slack.com/api/oauth.v2.access"

type SlackAuthService struct {
	cfg           config.SlackConfig
	workspaceRepo *repository.WorkspaceRepository
	httpClient    *http.Client
}

type SlackOAuthResult struct {
	WorkspaceID string `json:"workspace_id"`
	TeamID      string `json:"team_id"`
	TeamName    string `json:"team_name"`
	BotUserID   string `json:"bot_user_id"`
	Scope       string `json:"scope"`
}

type slackOAuthAccessResponse struct {
	OK          bool   `json:"ok"`
	Error       string `json:"error"`
	AccessToken string `json:"access_token"`
	Scope       string `json:"scope"`
	BotUserID   string `json:"bot_user_id"`
	Team        struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"team"`
	AuthedUser struct {
		ID string `json:"id"`
	} `json:"authed_user"`
}

func NewSlackAuthService(cfg config.SlackConfig, workspaceRepo *repository.WorkspaceRepository) *SlackAuthService {
	return &SlackAuthService{
		cfg:           cfg,
		workspaceRepo: workspaceRepo,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *SlackAuthService) InstallURL(state string) (string, error) {
	if strings.TrimSpace(s.cfg.ClientID) == "" {
		return "", fmt.Errorf("SLACK_CLIENT_ID is required")
	}
	if strings.TrimSpace(s.cfg.RedirectURL) == "" {
		return "", fmt.Errorf("SLACK_REDIRECT_URL is required")
	}

	if strings.TrimSpace(state) == "" {
		state = fmt.Sprintf("state-%d", time.Now().UnixNano())
	}

	botScopes := strings.TrimSpace(s.cfg.BotScopes)
	if botScopes == "" {
		botScopes = "chat:write,channels:read,users:read"
	}

	q := url.Values{}
	q.Set("client_id", s.cfg.ClientID)
	q.Set("scope", botScopes)
	q.Set("redirect_uri", s.cfg.RedirectURL)
	q.Set("state", state)
	if strings.TrimSpace(s.cfg.UserScopes) != "" {
		q.Set("user_scope", strings.TrimSpace(s.cfg.UserScopes))
	}

	return "https://slack.com/oauth/v2/authorize?" + q.Encode(), nil
}

func (s *SlackAuthService) ExchangeCode(ctx context.Context, code string) (SlackOAuthResult, error) {
	if strings.TrimSpace(s.cfg.ClientID) == "" {
		return SlackOAuthResult{}, fmt.Errorf("SLACK_CLIENT_ID is required")
	}
	if strings.TrimSpace(s.cfg.ClientSecret) == "" {
		return SlackOAuthResult{}, fmt.Errorf("SLACK_CLIENT_SECRET is required")
	}
	if strings.TrimSpace(s.cfg.RedirectURL) == "" {
		return SlackOAuthResult{}, fmt.Errorf("SLACK_REDIRECT_URL is required")
	}

	form := url.Values{}
	form.Set("client_id", s.cfg.ClientID)
	form.Set("client_secret", s.cfg.ClientSecret)
	form.Set("code", strings.TrimSpace(code))
	form.Set("redirect_uri", s.cfg.RedirectURL)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, slackOAuthAccessURL, strings.NewReader(form.Encode()))
	if err != nil {
		return SlackOAuthResult{}, fmt.Errorf("build oauth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return SlackOAuthResult{}, fmt.Errorf("exchange oauth code: %w", err)
	}
	defer resp.Body.Close()

	var payload slackOAuthAccessResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return SlackOAuthResult{}, fmt.Errorf("decode oauth response: %w", err)
	}

	if !payload.OK {
		if payload.Error == "" {
			payload.Error = "oauth exchange failed"
		}
		return SlackOAuthResult{}, fmt.Errorf("slack oauth error: %s", payload.Error)
	}

	if strings.TrimSpace(payload.Team.ID) == "" {
		return SlackOAuthResult{}, fmt.Errorf("oauth response missing team id")
	}

	workspace, err := s.workspaceRepo.SaveSlackInstallation(ctx, repository.SaveSlackInstallationInput{
		TeamID:          payload.Team.ID,
		TeamName:        payload.Team.Name,
		BotToken:        payload.AccessToken,
		BotUserID:       payload.BotUserID,
		InstallerUserID: payload.AuthedUser.ID,
		Scope:           payload.Scope,
	})
	if err != nil {
		return SlackOAuthResult{}, err
	}

	return SlackOAuthResult{
		WorkspaceID: workspace.ID,
		TeamID:      payload.Team.ID,
		TeamName:    payload.Team.Name,
		BotUserID:   payload.BotUserID,
		Scope:       payload.Scope,
	}, nil
}
