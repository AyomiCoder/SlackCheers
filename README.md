# SlackCheers

SlackCheers is a focused Slack celebration bot for birthdays and work anniversaries.

Current scope:
- Birthday celebrations
- Work anniversary celebrations
- Workspace/channel-level posting configuration
- People/date management via dashboard APIs
- Daily scheduler posting to configured Slack channels
- No billing or plan gating (everything is free for now)

## Tech stack

- Go + Gin (HTTP API)
- PostgreSQL (source of truth)
- Built-in SQL migration runner (Postgres)
- Air (hot reload in development)

## Quick start

1. Create a PostgreSQL database named `slackcheers`.
2. Copy env template:

```bash
cp .env.example .env
```

3. Install tools and dependencies:

```bash
make tools
make deps
```

4. Apply migrations:

```bash
make migrate-up
```

5. Run API with hot reload:

```bash
make dev
```

## Runtime migration behavior

On API boot (`cmd/api`), migrations run automatically when `MIGRATIONS_AUTO_APPLY=true`.

## Makefile workflow

- `make migration name=add_new_table` to create migration file
- `make migrate-up` to apply migrations
- `make migrate-down` to rollback one migration
- `make migrate-status` to inspect migration status
- `make test` to run tests
- `make lint` to run formatting check + vet

## API routes (MVP)

- `GET /healthz`
- `GET /auth/slack/install`
- `GET /auth/slack/callback`
- `POST /slack/events`
- `POST /api/workspaces/bootstrap`
- `GET /api/workspaces/:workspaceID/overview?days=30&type=all`
- `GET /api/workspaces/:workspaceID/people`
- `PUT /api/workspaces/:workspaceID/people/:slackUserID`
- `GET /api/workspaces/:workspaceID/channels`
- `GET /api/workspaces/:workspaceID/slack/channels`
- `PUT /api/workspaces/:workspaceID/channels/:channelID/settings`
- `PUT /api/workspaces/:workspaceID/channels/:channelID/templates`

## Swagger docs

Generate OpenAPI docs:

```bash
make swagger
```

Then run the API and open:

- `http://localhost:9060/swagger/index.html`

## Notes

- Celebration message posting still uses a no-op Slack client in `internal/slack`; OAuth install, callback, events verification, and channel discovery endpoints call Slack APIs directly.
- Scheduler uses exact day matching for weekends (no weekend carry-forward), matching MVP scope.

See docs:
- `docs/admin.md`
- `docs/developer.md`
