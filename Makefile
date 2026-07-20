.PHONY: run debug build start clean test sqlc up down down-v restart logs ps psql \
        docker-build docker-run \
        migrate-up migrate-down migrate-down-all migrate-force migrate-version migrate-create

SQLC_VERSION    := v1.29.0
MIGRATE_VERSION := v4.17.1
MIGRATIONS_DIR  := internal/db/migrations
BIN_DIR         := bin

IMAGE    := go-notifier-bot
VERSION  ?= dev
ENV_FILE ?= .env.local

# The migrator resolves the database from DATABASE_URL, falling back to .env.local
# and then to the local docker compose database. Override per invocation with
# `make migrate-up DATABASE_URL=postgres://...`.
DATABASE_URL ?=
export DATABASE_URL

MIGRATE := go run ./cmd/migrate -path $(MIGRATIONS_DIR)

run: ## Run the bot
	go run ./cmd/bot

debug: ## Run the bot with verbose debug logging
	LOG_LEVEL=debug go run ./cmd/bot

build: ## Build the bot and migrate binaries into bin/
	go build -o $(BIN_DIR)/bot ./cmd/bot
	go build -o $(BIN_DIR)/migrate ./cmd/migrate

start: build ## Build the bot, then run the compiled binary
	$(BIN_DIR)/bot

clean: ## Remove built binaries
	rm -rf $(BIN_DIR)

test: ## Run the tests
	go test ./...

sqlc: ## Generate Go code from SQL (migrations/*.up.sql + queries.sql) — update sqlc.yaml when you add a migration
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate

docker-build: ## Build the runtime image (make docker-build VERSION=1.2.3)
	docker build --build-arg VERSION=$(VERSION) -t $(IMAGE):$(VERSION) .

docker-run: docker-build ## Run the image locally with config from .env.local (make docker-run ENV_FILE=.env.other)
	@test -f $(ENV_FILE) || { echo "$(ENV_FILE) not found — copy .env.example to .env.local first"; exit 1; }
	@# .env.local is passed at runtime, never baked into the image (see .dockerignore).
	@# A DATABASE_URL pointing at the host's Postgres is unreachable from inside the
	@# container, so rewrite that host only; a remote URL is left untouched.
	@url=$$(grep -E '^DATABASE_URL=' $(ENV_FILE) | tail -1 | cut -d= -f2- | tr -d '"'"'"'"'); \
	url=$${DATABASE_URL:-$${url:-postgres://postgres:postgres@localhost:5432/notifier?sslmode=disable}}; \
	url=$$(printf '%s' "$$url" | sed -e 's#@localhost:#@host.docker.internal:#' -e 's#@127\.0\.0\.1:#@host.docker.internal:#'); \
	echo "running $(IMAGE):$(VERSION) with env from $(ENV_FILE), db host $$(printf '%s' "$$url" | sed -e 's#.*@##' -e 's#[/?].*##')"; \
	docker run --rm -i $$([ -t 0 ] && printf -- -t) \
	  --env-file $(ENV_FILE) \
	  --add-host host.docker.internal:host-gateway \
	  -e DATABASE_URL="$$url" \
	  $(IMAGE):$(VERSION)

up: ## Start local Postgres + Adminer, then apply migrations
	docker compose up -d
	@echo "waiting for db..."
	@until docker compose exec -T db pg_isready -U postgres -d notifier >/dev/null 2>&1; do sleep 1; done
	$(MAKE) migrate-up

down: ## Stop local containers (keep data)
	docker compose down

down-v: ## Stop local containers and wipe the DB volume
	docker compose down -v

restart: down-v up ## Wipe the local DB and rebuild it from migrations

logs: ## Tail container logs
	docker compose logs -f

ps: ## Show container status
	docker compose ps

psql: ## Open a psql shell into the local DB
	docker compose exec db psql -U postgres -d notifier

migrate-up: ## Apply all pending migrations
	$(MIGRATE) up

migrate-down: ## Roll back the last migration
	$(MIGRATE) down 1

migrate-force: ## Force the schema_migrations version without running SQL (make migrate-force VERSION=1)
	$(MIGRATE) force $(VERSION)

migrate-down-all: ## Roll back every migration
	$(MIGRATE) down all

migrate-version: ## Show the current migration version
	$(MIGRATE) version

migrate-create: ## Create a new migration pair (make migrate-create name=add_users)
	go run github.com/golang-migrate/migrate/v4/cmd/migrate@$(MIGRATE_VERSION) create -ext sql -dir $(MIGRATIONS_DIR) -seq $(name)
