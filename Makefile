.PHONY: run sqlc up down down-v restart logs ps psql \
        migrate-up migrate-down migrate-force migrate-version migrate-create

SQLC_VERSION    := v1.27.0
MIGRATE_VERSION := v4.17.1
MIGRATIONS_DIR  := internal/db/migrations
DB_URL          ?= postgres://postgres:postgres@localhost:5432/notifier?sslmode=disable

MIGRATE := go run -tags 'postgres,file' github.com/golang-migrate/migrate/v4/cmd/migrate@$(MIGRATE_VERSION) \
	-path $(MIGRATIONS_DIR) -database "$(DB_URL)"

run: ## Run the bot
	go run ./cmd

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

migrate-version: ## Show the current migration version
	$(MIGRATE) version

migrate-create: ## Create a new migration pair (make migrate-create name=add_users)
	go run github.com/golang-migrate/migrate/v4/cmd/migrate@$(MIGRATE_VERSION) create -ext sql -dir $(MIGRATIONS_DIR) -seq $(name)
