.PHONY: run debug test sqlc up down down-v restart logs ps psql \
        migrate-up migrate-down migrate-down-all migrate-force migrate-version migrate-create

SQLC_VERSION    := v1.29.0
MIGRATE_VERSION := v4.17.1
MIGRATIONS_DIR  := internal/db/migrations

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

test: ## Run the tests
	go test ./...

sqlc: ## Generate Go code from SQL (migrations/*.up.sql + queries.sql) — update sqlc.yaml when you add a migration
	go run github.com/sqlc-dev/sqlc/cmd/sqlc@$(SQLC_VERSION) generate

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
