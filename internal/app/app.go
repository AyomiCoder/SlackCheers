package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"slackcheers/internal/config"
	"slackcheers/internal/database"
	apphttp "slackcheers/internal/http"
	"slackcheers/internal/http/handlers"
	"slackcheers/internal/repository"
	"slackcheers/internal/scheduler"
	"slackcheers/internal/service"
	"slackcheers/internal/slack"
)

type App struct {
	cfg       config.Config
	logger    *slog.Logger
	db        *sql.DB
	httpSrv   *http.Server
	scheduler *scheduler.Scheduler
}

func New(ctx context.Context) (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	logger := newLogger(cfg.App.Environment)

	db, err := database.OpenPostgres(ctx, cfg.DB)
	if err != nil {
		return nil, err
	}

	if cfg.DB.AutoMigrate {
		if err := database.UpMigrations(ctx, db, cfg.DB.MigrationsDir); err != nil {
			_ = db.Close()
			return nil, err
		}
		logger.Info("database migrations applied", slog.String("dir", cfg.DB.MigrationsDir))
	}

	workspaceRepo := repository.NewWorkspaceRepository(db)
	peopleRepo := repository.NewPeopleRepository(db)
	slackClient, err := slack.NewClient(cfg.Slack.BotToken, logger)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("build slack client: %w", err)
	}

	celebrationSvc := service.NewCelebrationService(workspaceRepo, peopleRepo, slackClient, logger)
	dashboardSvc := service.NewDashboardService(workspaceRepo, peopleRepo)

	healthHandler := handlers.NewHealthHandler()
	workspaceHandler := handlers.NewWorkspaceHandler(dashboardSvc, workspaceRepo)

	router := apphttp.NewRouter(apphttp.RouterDependencies{
		Logger:           logger,
		HealthHandler:    healthHandler,
		WorkspaceHandler: workspaceHandler,
	})

	httpSrv := &http.Server{
		Addr:              ":" + cfg.Server.Port,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	var sched *scheduler.Scheduler
	if cfg.Scheduler.Enabled {
		sched = scheduler.New(celebrationSvc, cfg.Scheduler.PollInterval, logger)
	}

	return &App{
		cfg:       cfg,
		logger:    logger,
		db:        db,
		httpSrv:   httpSrv,
		scheduler: sched,
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if a.scheduler != nil {
		go a.scheduler.Run(ctx)
	}

	errCh := make(chan error, 1)
	go func() {
		a.logger.Info("http server starting", slog.String("addr", a.httpSrv.Addr))
		if err := a.httpSrv.ListenAndServe(); err != nil {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		return a.shutdown(context.Background())
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return a.shutdown(context.Background())
		}
		_ = a.shutdown(context.Background())
		return err
	}
}

func (a *App) shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := a.httpSrv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}

	if err := a.db.Close(); err != nil {
		return fmt.Errorf("close db: %w", err)
	}

	return nil
}
