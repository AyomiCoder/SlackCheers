package http

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"slackcheers/internal/http/handlers"
	"slackcheers/internal/http/middleware"
)

type RouterDependencies struct {
	Logger           *slog.Logger
	HealthHandler    *handlers.HealthHandler
	WorkspaceHandler *handlers.WorkspaceHandler
}

func NewRouter(deps RouterDependencies) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestLogger(deps.Logger))

	r.GET("/healthz", deps.HealthHandler.Healthz)
	r.GET("/auth/slack/callback", deps.WorkspaceHandler.SlackOAuthCallback)

	api := r.Group("/api")
	{
		api.POST("/workspaces/bootstrap", deps.WorkspaceHandler.BootstrapWorkspace)
		api.GET("/workspaces/:workspaceID/overview", deps.WorkspaceHandler.Overview)
		api.GET("/workspaces/:workspaceID/people", deps.WorkspaceHandler.ListPeople)
		api.PUT("/workspaces/:workspaceID/people/:slackUserID", deps.WorkspaceHandler.UpsertPerson)
		api.GET("/workspaces/:workspaceID/channels", deps.WorkspaceHandler.ListChannels)
		api.PUT("/workspaces/:workspaceID/channels/:channelID/settings", deps.WorkspaceHandler.UpdateChannelSettings)
		api.PUT("/workspaces/:workspaceID/channels/:channelID/templates", deps.WorkspaceHandler.UpdateChannelTemplates)
	}

	return r
}
