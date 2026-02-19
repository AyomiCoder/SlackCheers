package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"slackcheers/internal/service"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService    *service.SlackAuthService
	inboundService *service.SlackInboundService
	signingSecret  string
}

func NewAuthHandler(
	authService *service.SlackAuthService,
	inboundService *service.SlackInboundService,
	signingSecret string,
) *AuthHandler {
	return &AuthHandler{
		authService:    authService,
		inboundService: inboundService,
		signingSecret:  strings.TrimSpace(signingSecret),
	}
}

// SlackInstall godoc
// @Summary Start Slack install
// @Description Redirects to Slack OAuth consent page. Use mode=json to return URL without redirect.
// @Tags auth
// @Produce json
// @Param state query string false "Opaque CSRF state"
// @Param mode query string false "Set to json to return install URL"
// @Success 200 {object} SlackInstallURLResponse
// @Success 307 {string} string "Temporary Redirect"
// @Failure 500 {object} ErrorResponse
// @Router /auth/slack/install [get]
func (h *AuthHandler) SlackInstall(c *gin.Context) {
	state := strings.TrimSpace(c.Query("state"))
	installURL, err := h.authService.InstallURL(state)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if strings.EqualFold(strings.TrimSpace(c.Query("mode")), "json") {
		c.JSON(http.StatusOK, SlackInstallURLResponse{
			InstallURL: installURL,
			State:      state,
		})
		return
	}

	c.Redirect(http.StatusTemporaryRedirect, installURL)
}

// SlackOAuthCallback godoc
// @Summary Slack OAuth callback
// @Description Exchanges OAuth code, stores workspace install metadata, and returns connected workspace details.
// @Tags auth
// @Produce json
// @Param code query string true "Slack OAuth code"
// @Param state query string false "Opaque CSRF state"
// @Param error query string false "Slack OAuth error"
// @Success 200 {object} SlackConnectResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /auth/slack/callback [get]
func (h *AuthHandler) SlackOAuthCallback(c *gin.Context) {
	if oauthErr := strings.TrimSpace(c.Query("error")); oauthErr != "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slack oauth denied: " + oauthErr})
		return
	}

	code := strings.TrimSpace(c.Query("code"))
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing oauth code"})
		return
	}

	result, err := h.authService.ExchangeCode(c.Request.Context(), code)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, SlackConnectResponse{
		Status: "connected",
		Installation: SlackOAuthInstallation{
			WorkspaceID: result.WorkspaceID,
			TeamID:      result.TeamID,
			TeamName:    result.TeamName,
			BotUserID:   result.BotUserID,
			Scope:       result.Scope,
		},
	})
}

// SlackEvents godoc
// @Summary Slack events webhook
// @Description Verifies Slack signatures, handles URL verification, and processes DM replies to save birthdays/hire dates.
// @Tags slack
// @Accept json
// @Produce json
// @Param payload body SlackEventEnvelope true "Slack event payload"
// @Success 200 {object} SlackEventAckResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /slack/events [post]
func (h *AuthHandler) SlackEvents(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	if strings.TrimSpace(h.signingSecret) == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "SLACK_SIGNING_SECRET is required for events endpoint"})
		return
	}

	timestamp := c.GetHeader("X-Slack-Request-Timestamp")
	signature := c.GetHeader("X-Slack-Signature")
	if !isValidSlackSignature(h.signingSecret, timestamp, signature, body) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid slack signature"})
		return
	}

	var payload SlackEventEnvelope
	if err := json.Unmarshal(body, &payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid json payload"})
		return
	}

	if payload.Type == "url_verification" {
		c.JSON(http.StatusOK, SlackEventAckResponse{Challenge: payload.Challenge})
		return
	}

	if h.inboundService != nil {
		_ = h.inboundService.ProcessEvent(c.Request.Context(), body)
	}

	c.JSON(http.StatusOK, SlackEventAckResponse{OK: true})
}

func isValidSlackSignature(signingSecret, timestamp, signature string, body []byte) bool {
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}

	now := time.Now().Unix()
	if now-ts > 60*5 || ts-now > 60*5 {
		return false
	}

	base := "v0:" + timestamp + ":" + string(body)
	mac := hmac.New(sha256.New, []byte(signingSecret))
	_, _ = mac.Write([]byte(base))
	expected := "v0=" + hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(signature))
}
