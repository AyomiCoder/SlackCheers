package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"slackcheers/internal/repository"
	"slackcheers/internal/service"
)

type WorkspaceHandler struct {
	dashboardSvc  *service.DashboardService
	workspaceRepo *repository.WorkspaceRepository
}

func NewWorkspaceHandler(dashboardSvc *service.DashboardService, workspaceRepo *repository.WorkspaceRepository) *WorkspaceHandler {
	return &WorkspaceHandler{dashboardSvc: dashboardSvc, workspaceRepo: workspaceRepo}
}

type bootstrapWorkspaceRequest struct {
	SlackTeamID string `json:"slack_team_id" binding:"required"`
	Name        string `json:"name" binding:"required"`
	Timezone    string `json:"timezone" binding:"required"`
	ChannelID   string `json:"channel_id" binding:"required"`
	ChannelName string `json:"channel_name" binding:"required"`
	PostingTime string `json:"posting_time" binding:"required"`
}

func (h *WorkspaceHandler) BootstrapWorkspace(c *gin.Context) {
	var req bootstrapWorkspaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if _, err := time.LoadLocation(req.Timezone); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid timezone"})
		return
	}

	if _, err := time.Parse("15:04", req.PostingTime); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "posting_time must use HH:MM"})
		return
	}

	workspace, err := h.workspaceRepo.EnsureWorkspace(c.Request.Context(), req.SlackTeamID, req.Name, req.Timezone)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	channel, err := h.workspaceRepo.CreateDefaultChannel(c.Request.Context(), workspace.ID, req.ChannelID, req.ChannelName, req.Timezone, req.PostingTime)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"workspace": workspace,
		"channel":   channel,
	})
}

func (h *WorkspaceHandler) Overview(c *gin.Context) {
	workspaceID := c.Param("workspaceID")
	days := 30
	if rawDays := strings.TrimSpace(c.Query("days")); rawDays != "" {
		parsed, err := strconv.Atoi(rawDays)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "days must be a number"})
			return
		}
		days = parsed
	}

	celebrationType := strings.ToLower(strings.TrimSpace(c.DefaultQuery("type", "all")))
	if celebrationType != "all" && celebrationType != "birthdays" && celebrationType != "anniversaries" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type must be one of all|birthdays|anniversaries"})
		return
	}

	items, err := h.dashboardSvc.Overview(c.Request.Context(), workspaceID, days, celebrationType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *WorkspaceHandler) ListPeople(c *gin.Context) {
	workspaceID := c.Param("workspaceID")
	people, err := h.dashboardSvc.ListPeople(c.Request.Context(), workspaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"people": people})
}

type upsertPersonRequest struct {
	SlackHandle            string `json:"slack_handle" binding:"required"`
	DisplayName            string `json:"display_name" binding:"required"`
	AvatarURL              string `json:"avatar_url"`
	BirthdayDay            *int   `json:"birthday_day"`
	BirthdayMonth          *int   `json:"birthday_month"`
	BirthdayYear           *int   `json:"birthday_year"`
	HireDate               string `json:"hire_date"`
	PublicCelebrationOptIn *bool  `json:"public_celebration_opt_in"`
	RemindersMode          string `json:"reminders_mode"`
}

func (h *WorkspaceHandler) UpsertPerson(c *gin.Context) {
	workspaceID := c.Param("workspaceID")
	slackUserID := c.Param("slackUserID")

	var req upsertPersonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var hireDate *time.Time
	if strings.TrimSpace(req.HireDate) != "" {
		parsed, err := time.Parse("2006-01-02", req.HireDate)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "hire_date must use YYYY-MM-DD"})
			return
		}
		hireDate = &parsed
	}

	mode := strings.TrimSpace(req.RemindersMode)
	if mode == "" {
		mode = "same_day"
	}
	if mode != "none" && mode != "same_day" && mode != "day_before" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "reminders_mode must be none|same_day|day_before"})
		return
	}

	publicCelebrationOptIn := true
	if req.PublicCelebrationOptIn != nil {
		publicCelebrationOptIn = *req.PublicCelebrationOptIn
	}

	person, err := h.dashboardSvc.UpsertPerson(c.Request.Context(), repository.UpsertPersonInput{
		WorkspaceID:            workspaceID,
		SlackUserID:            slackUserID,
		SlackHandle:            req.SlackHandle,
		DisplayName:            req.DisplayName,
		AvatarURL:              req.AvatarURL,
		BirthdayDay:            req.BirthdayDay,
		BirthdayMonth:          req.BirthdayMonth,
		BirthdayYear:           req.BirthdayYear,
		HireDate:               hireDate,
		PublicCelebrationOptIn: publicCelebrationOptIn,
		RemindersMode:          mode,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, person)
}

func (h *WorkspaceHandler) ListChannels(c *gin.Context) {
	workspaceID := c.Param("workspaceID")
	channels, err := h.dashboardSvc.ListChannels(c.Request.Context(), workspaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"channels": channels})
}

type updateChannelSettingsRequest struct {
	PostingTime          string `json:"posting_time" binding:"required"`
	Timezone             string `json:"timezone" binding:"required"`
	BirthdaysEnabled     *bool  `json:"birthdays_enabled" binding:"required"`
	AnniversariesEnabled *bool  `json:"anniversaries_enabled" binding:"required"`
}

func (h *WorkspaceHandler) UpdateChannelSettings(c *gin.Context) {
	workspaceID := c.Param("workspaceID")
	channelID := c.Param("channelID")

	var req updateChannelSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	channel, err := h.dashboardSvc.UpdateChannelSettings(
		c.Request.Context(),
		workspaceID,
		channelID,
		req.PostingTime,
		req.Timezone,
		*req.BirthdaysEnabled,
		*req.AnniversariesEnabled,
	)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, channel)
}

type updateChannelTemplatesRequest struct {
	BirthdayTemplate    string `json:"birthday_template" binding:"required"`
	AnniversaryTemplate string `json:"anniversary_template" binding:"required"`
	BrandingEmoji       string `json:"branding_emoji"`
}

func (h *WorkspaceHandler) UpdateChannelTemplates(c *gin.Context) {
	workspaceID := c.Param("workspaceID")
	channelID := c.Param("channelID")

	var req updateChannelTemplatesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	channel, err := h.dashboardSvc.UpdateChannelTemplates(
		c.Request.Context(),
		workspaceID,
		channelID,
		req.BirthdayTemplate,
		req.AnniversaryTemplate,
		req.BrandingEmoji,
	)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "channel not found"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, channel)
}

func (h *WorkspaceHandler) SlackOAuthCallback(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{
		"message": "Slack OAuth callback endpoint is reserved; implement OAuth exchange next",
	})
}
