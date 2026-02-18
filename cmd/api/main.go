package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"slackcheers/internal/app"
)

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
