package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"slackcheers/internal/repository"
	"slackcheers/internal/service"

	"github.com/gin-gonic/gin"
)

type WorkspaceHandler struct {
	dashboardSvc  *service.DashboardService
	onboardingSvc *service.SlackOnboardingService
	slackChannels *service.SlackChannelsService
	workspaceRepo *repository.WorkspaceRepository
}

func NewWorkspaceHandler(
	dashboardSvc *service.DashboardService,
	onboardingSvc *service.SlackOnboardingService,
	slackChannels *service.SlackChannelsService,
	workspaceRepo *repository.WorkspaceRepository,
) *WorkspaceHandler {
	return &WorkspaceHandler{
		dashboardSvc:  dashboardSvc,
		onboardingSvc: onboardingSvc,
		slackChannels: slackChannels,
		workspaceRepo: workspaceRepo,
	}
}

// BootstrapWorkspace godoc
// @Summary Bootstrap a workspace
// @Description Creates or updates a workspace and its default celebration channel.
// @Tags workspaces
// @Accept json
// @Produce json
// @Param request body BootstrapWorkspaceRequest true "Workspace bootstrap payload"
// @Success 201 {object} BootstrapWorkspaceResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/workspaces/bootstrap [post]
func (h *WorkspaceHandler) BootstrapWorkspace(c *gin.Context) {
	var req BootstrapWorkspaceRequest
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

// Overview godoc
// @Summary List upcoming celebrations
// @Description Returns upcoming birthdays and/or anniversaries for a workspace.
// @Tags workspaces
// @Produce json
// @Param workspaceID path string true "Workspace ID"
// @Param days query int false "Number of days to include (default 30)"
// @Param type query string false "Filter: all|birthdays|anniversaries"
// @Success 200 {object} OverviewResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/workspaces/{workspaceID}/overview [get]
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

// ListPeople godoc
// @Summary List people in a workspace
// @Tags people
// @Produce json
// @Param workspaceID path string true "Workspace ID"
// @Success 200 {object} PeopleResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/workspaces/{workspaceID}/people [get]
func (h *WorkspaceHandler) ListPeople(c *gin.Context) {
	workspaceID := c.Param("workspaceID")
	people, err := h.dashboardSvc.ListPeople(c.Request.Context(), workspaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"people": people})
}

// UpsertPerson godoc
// @Summary Create or update a person
// @Tags people
// @Accept json
// @Produce json
// @Param workspaceID path string true "Workspace ID"
// @Param slackUserID path string true "Slack User ID"
// @Param request body UpsertPersonRequest true "Person payload"
// @Success 200 {object} slackcheers_internal_domain.Person
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/workspaces/{workspaceID}/people/{slackUserID} [put]
func (h *WorkspaceHandler) UpsertPerson(c *gin.Context) {
	workspaceID := c.Param("workspaceID")
	slackUserID := c.Param("slackUserID")

	var req UpsertPersonRequest
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

// ListChannels godoc
// @Summary List workspace channels
// @Tags channels
// @Produce json
// @Param workspaceID path string true "Workspace ID"
// @Success 200 {object} ChannelsResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/workspaces/{workspaceID}/channels [get]
func (h *WorkspaceHandler) ListChannels(c *gin.Context) {
	workspaceID := c.Param("workspaceID")
	channels, err := h.dashboardSvc.ListChannels(c.Request.Context(), workspaceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"channels": channels})
}

// SendOnboardingDMs godoc
// @Summary Send onboarding DMs to workspace members
// @Description Sends one onboarding DM per member (once only), asking for birthday and work start date.
// @Tags onboarding
// @Produce json
// @Param workspaceID path string true "Workspace ID"
// @Success 200 {object} OnboardingDMDispatchResponse
// @Failure 404 {object} ErrorResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/workspaces/{workspaceID}/onboarding/dm [post]
func (h *WorkspaceHandler) SendOnboardingDMs(c *gin.Context) {
	workspaceID := c.Param("workspaceID")
	result, err := h.onboardingSvc.SendOnboardingDMs(c.Request.Context(), workspaceID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "workspace not found"})
			return
		}
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "not connected") || strings.Contains(msg, "slack api error") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, OnboardingDMDispatchResponse{
		TotalMembers:  result.TotalMembers,
		Sent:          result.Sent,
		Skipped:       result.Skipped,
		Failed:        result.Failed,
		FailedUsers:   result.FailedUsers,
		FailedDetails: result.FailedDetails,
	})
}

// ListSlackChannels godoc
// @Summary List Slack channels for workspace connection
// @Description Fetches channels directly from Slack using the workspace-installed bot token.
// @Tags channels
// @Produce json
// @Param workspaceID path string true "Workspace ID"
// @Success 200 {object} SlackChannelsResponse
// @Failure 404 {object} ErrorResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/workspaces/{workspaceID}/slack/channels [get]
func (h *WorkspaceHandler) ListSlackChannels(c *gin.Context) {
	workspaceID := c.Param("workspaceID")
	channels, err := h.slackChannels.ListChannels(c.Request.Context(), workspaceID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "workspace not found"})
			return
		}
		if strings.Contains(strings.ToLower(err.Error()), "not connected") {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	items := make([]SlackChannelItem, 0, len(channels))
	for _, ch := range channels {
		items = append(items, SlackChannelItem{
			ID:        ch.ID,
			Name:      ch.Name,
			IsPrivate: ch.IsPrivate,
		})
	}

	c.JSON(http.StatusOK, SlackChannelsResponse{Channels: items})
}

// UpdateChannelSettings godoc
// @Summary Update channel settings
// @Tags channels
// @Accept json
// @Produce json
// @Param workspaceID path string true "Workspace ID"
// @Param channelID path string true "Channel ID"
// @Param request body UpdateChannelSettingsRequest true "Channel settings payload"
// @Success 200 {object} slackcheers_internal_domain.WorkspaceChannel
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/workspaces/{workspaceID}/channels/{channelID}/settings [put]
func (h *WorkspaceHandler) UpdateChannelSettings(c *gin.Context) {
	workspaceID := c.Param("workspaceID")
	channelID := c.Param("channelID")

	var req UpdateChannelSettingsRequest
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

// UpdateChannelTemplates godoc
// @Summary Update channel templates
// @Tags channels
// @Accept json
// @Produce json
// @Param workspaceID path string true "Workspace ID"
// @Param channelID path string true "Channel ID"
// @Param request body UpdateChannelTemplatesRequest true "Channel templates payload"
// @Success 200 {object} slackcheers_internal_domain.WorkspaceChannel
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/workspaces/{workspaceID}/channels/{channelID}/templates [put]
func (h *WorkspaceHandler) UpdateChannelTemplates(c *gin.Context) {
	workspaceID := c.Param("workspaceID")
	channelID := c.Param("channelID")

	var req UpdateChannelTemplatesRequest
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
