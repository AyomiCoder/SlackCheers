package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	_ "slackcheers/docs/swagger"
	"slackcheers/internal/app"
)

// @title SlackCheers API
// @version 1.0
// @description SlackCheers API for workspace setup, people management, channel settings, and celebrations.
// @BasePath /
// @schemes http https
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	application, err := app.New(ctx)
	if err != nil {
		log.Fatalf("failed to initialize app: %v", err)
	}

	if err := application.Run(ctx); err != nil {
		log.Fatalf("application stopped with error: %v", err)
	}
}
