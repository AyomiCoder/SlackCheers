package scheduler

import (
	"context"
	"log/slog"
	"time"

	"slackcheers/internal/service"
)

type Scheduler struct {
	service      *service.CelebrationService
	pollInterval time.Duration
	logger       *slog.Logger
}

func New(service *service.CelebrationService, pollInterval time.Duration, logger *slog.Logger) *Scheduler {
	return &Scheduler{
		service:      service,
		pollInterval: pollInterval,
		logger:       logger,
	}
}

func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()

	s.logger.Info("scheduler started", slog.Duration("poll_interval", s.pollInterval))
	for {
		select {
		case <-ctx.Done():
			s.logger.Info("scheduler stopped")
			return
		case now := <-ticker.C:
			if err := s.service.RunDueCelebrations(ctx, now.UTC()); err != nil {
				s.logger.Error("scheduler tick failed", slog.String("error", err.Error()))
			}
		}
	}
}
