SHELL := /bin/zsh

APP_NAME := slackcheers-api
APP_BIN := bin/$(APP_NAME)

.PHONY: help tools deps dev run build test fmt vet lint migration migrate-up migrate-down migrate-status clean

help:
	@echo "Available targets:"
	@echo "  make tools            # install local dev tools (air)"
	@echo "  make deps             # tidy go modules"
	@echo "  make dev              # run API with air hot reload"
	@echo "  make run              # run API directly"
	@echo "  make build            # build API binary"
	@echo "  make test             # run test suite"
	@echo "  make fmt              # format go files"
	@echo "  make vet              # run go vet"
	@echo "  make lint             # fmt check + vet"
	@echo "  make migration name=create_people_table"
	@echo "  make migrate-up       # apply migrations"
	@echo "  make migrate-down     # rollback 1 migration"
	@echo "  make migrate-status   # print migration status"
	@echo "  make clean            # remove build artifacts"

tools:
	go install github.com/air-verse/air@latest

deps:
	go mod tidy

dev:
	air -c .air.toml

run:
	go run ./cmd/api

build:
	mkdir -p bin
	go build -o $(APP_BIN) ./cmd/api

test:
	go test ./...

fmt:
	@files=$$(rg --files -g '*.go'); \
	if [ -n "$$files" ]; then gofmt -w $$files; fi

vet:
	go vet ./...

lint:
	@unformatted=$$(gofmt -l $$(rg --files -g '*.go')); \
	if [ -n "$$unformatted" ]; then \
		echo "Unformatted files:"; echo "$$unformatted"; exit 1; \
	fi
	go vet ./...

migration:
	@test -n "$(name)" || (echo "usage: make migration name=create_people_table" && exit 1)
	@version=$$(date +%s); \
	up=db/migrations/$${version}_$(name).up.sql; \
	down=db/migrations/$${version}_$(name).down.sql; \
	touch "$$up" "$$down"; \
	echo "created $$up"; \
	echo "created $$down"

migrate-up:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down

migrate-status:
	go run ./cmd/migrate status

clean:
	rm -rf bin tmp
