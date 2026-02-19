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
- `POST /api/workspaces/:workspaceID/dispatch-now`
- `GET /api/workspaces/:workspaceID/overview?days=30&type=all`
- `GET /api/workspaces/:workspaceID/people`
- `PUT /api/workspaces/:workspaceID/people/:slackUserID`
- `GET /api/workspaces/:workspaceID/channels`
- `POST /api/workspaces/:workspaceID/channels/:channelID/cleanup-birthday-messages`
- `GET /api/workspaces/:workspaceID/slack/channels`
- `POST /api/workspaces/:workspaceID/onboarding/dm`
- `POST /api/workspaces/:workspaceID/onboarding/dm/cleanup?user_id=U123`
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

- Slack API is used for OAuth install/callback, events verification, channel discovery, onboarding DMs, and celebration posting.
- Team members can reply in bot DM with one or both lines to save data:
  ```text
  march 25
  january 23, 2024
  ```
  `month day` saves birthday, `month day, year` saves hire date (year required).
- Scheduler uses exact day matching for weekends (no weekend carry-forward), matching MVP scope.

See docs:
- `docs/admin.md`
- `docs/developer.md`
