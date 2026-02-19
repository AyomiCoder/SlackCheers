package http

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"slackcheers/internal/http/handlers"
	"slackcheers/internal/http/middleware"
)

type RouterDependencies struct {
	Logger           *slog.Logger
	HealthHandler    *handlers.HealthHandler
	AuthHandler      *handlers.AuthHandler
	WorkspaceHandler *handlers.WorkspaceHandler
}

func NewRouter(deps RouterDependencies) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestLogger(deps.Logger))

	r.GET("/healthz", deps.HealthHandler.Healthz)
	r.GET("/auth/slack/install", deps.AuthHandler.SlackInstall)
	r.GET("/auth/slack/callback", deps.AuthHandler.SlackOAuthCallback)
	r.POST("/slack/events", deps.AuthHandler.SlackEvents)
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	api := r.Group("/api")
	{
		api.POST("/workspaces/bootstrap", deps.WorkspaceHandler.BootstrapWorkspace)
		api.POST("/workspaces/:workspaceID/dispatch-now", deps.WorkspaceHandler.DispatchCelebrationsNow)
		api.GET("/workspaces/:workspaceID/overview", deps.WorkspaceHandler.Overview)
		api.GET("/workspaces/:workspaceID/people", deps.WorkspaceHandler.ListPeople)
		api.PUT("/workspaces/:workspaceID/people/:slackUserID", deps.WorkspaceHandler.UpsertPerson)
		api.GET("/workspaces/:workspaceID/channels", deps.WorkspaceHandler.ListChannels)
		api.POST("/workspaces/:workspaceID/channels/:channelID/cleanup-birthday-messages", deps.WorkspaceHandler.CleanupBirthdayMessages)
		api.GET("/workspaces/:workspaceID/slack/channels", deps.WorkspaceHandler.ListSlackChannels)
		api.POST("/workspaces/:workspaceID/onboarding/dm", deps.WorkspaceHandler.SendOnboardingDMs)
		api.POST("/workspaces/:workspaceID/onboarding/dm/cleanup", deps.WorkspaceHandler.CleanupOnboardingDMs)
		api.PUT("/workspaces/:workspaceID/channels/:channelID/settings", deps.WorkspaceHandler.UpdateChannelSettings)
		api.PUT("/workspaces/:workspaceID/channels/:channelID/templates", deps.WorkspaceHandler.UpdateChannelTemplates)
	}

	return r
}
