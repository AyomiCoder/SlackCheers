# SlackCheers Developer Guide

## Architecture

- `cmd/api`: API process bootstrap
- `cmd/migrate`: migration CLI
- `internal/config`: environment config loader
- `internal/database`: Postgres setup + migration runner
- `internal/repository`: SQL data access
- `internal/service`: business logic for dashboard + celebrations
- `internal/scheduler`: periodic runner for daily celebration dispatch
- `internal/slack`: Slack client boundary
- `internal/http`: Gin router, middleware, handlers
- `db/migrations`: SQL schema migrations

## Environment variables

See `.env.example`.

Core values:
- `DATABASE_URL`
- `APP_PORT`
- `MIGRATIONS_AUTO_APPLY`
- `SCHEDULER_ENABLED`
- `SLACK_CLIENT_ID`
- `SLACK_CLIENT_SECRET`
- `SLACK_REDIRECT_URL`
- `SLACK_BOT_TOKEN`
- `SLACK_SIGNING_SECRET`

## Migrations

- Create: `make migration name=add_table_name`
- Apply: `make migrate-up`
- Rollback one: `make migrate-down`
- Status: `make migrate-status`

API startup also applies migrations when `MIGRATIONS_AUTO_APPLY=true`.

## Swagger

- Generate docs: `make swagger`
- Swagger UI: `http://localhost:9060/swagger/index.html`

## API contract (initial)

- `GET /auth/slack/install`
- `GET /auth/slack/callback`
- `POST /slack/events`
- `GET /api/workspaces/:workspaceID/overview`
- `GET /api/workspaces/:workspaceID/people`
- `PUT /api/workspaces/:workspaceID/people/:slackUserID`
- `GET /api/workspaces/:workspaceID/channels`
- `GET /api/workspaces/:workspaceID/slack/channels`
- `POST /api/workspaces/:workspaceID/onboarding/dm`
- `PUT /api/workspaces/:workspaceID/channels/:channelID/settings`
- `PUT /api/workspaces/:workspaceID/channels/:channelID/templates`

## Engineering principles used

- Clear boundaries between handlers/services/repositories
- Context-aware DB calls
- Startup migration safety
- Graceful HTTP shutdown
- Dependency injection for replaceable Slack client implementation
