WEB_DIR = ./web/default
WEB_CLASSIC_DIR = ./web/classic
API_DIR = .
DEV_ENV ?= .env.dev
BUN_VERSION ?= 1.3.14
BUN = npx --yes bun@$(BUN_VERSION)
DEV_WEB_DEFAULT_PORT ?= 3001
DEV_WEB_CLASSIC_PORT ?= 3002
DEV_COMPOSE_FILE = docker-compose.dev.yml
DEV_COMPOSE = docker compose --env-file $(DEV_ENV) -f $(DEV_COMPOSE_FILE)
DEV_POSTGRES_SERVICE = postgres
DEV_API_SERVICE = new-api
DEV_POSTGRES_DB = newapi
DEV_POSTGRES_USER = relay
DEV_SQLITE_PATH ?= one-api.db
PERSONAL_DEV_SCRIPT = node scripts/personal-dev.mjs

.PHONY: all build-web build-web-classic build-all-web start-api dev dev-init dev-bootstrap dev-api dev-api-rebuild dev-infra-up dev-infra-status dev-backend dev-web dev-frontend dev-web-classic dev-down dev-reset reset-setup personal-dev-init personal-dev-up personal-dev-sync personal-dev-rebuild personal-dev-status personal-dev-logs personal-dev-doctor personal-dev-down personal-dev-tunnel-up personal-dev-tunnel-down personal-dev-web-up personal-dev-web-status personal-dev-web-logs personal-dev-web-down

all: build-all-web start-api

dev-init:
	@test -f $(DEV_ENV) || cp .env.dev.example $(DEV_ENV)

dev-bootstrap: dev-init
	rm -rf web/node_modules
	cd web && $(BUN) install --filter ./classic --frozen-lockfile
	cd $(WEB_CLASSIC_DIR) && VITE_REACT_APP_VERSION=$$(cat ../../VERSION) $(BUN) run build
	rm -rf web/node_modules
	cd web && $(BUN) install --frozen-lockfile
	cd $(WEB_DIR) && VITE_REACT_APP_VERSION=$$(cat ../../VERSION) $(BUN) run build:check

build-web:
	@echo "Building default web..."
	@cd ./web && $(BUN) install --frozen-lockfile
	@cd $(WEB_DIR) && DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION=$$(cat ../../VERSION) $(BUN) run build

build-web-classic:
	@echo "Building classic web..."
	@rm -rf ./web/node_modules
	@cd ./web && $(BUN) install --filter ./classic --frozen-lockfile
	@cd $(WEB_CLASSIC_DIR) && VITE_REACT_APP_VERSION=$$(cat ../../VERSION) $(BUN) run build

build-all-web: build-web build-web-classic

start-api:
	@echo "Starting api dev server..."
	@cd $(API_DIR) && go run main.go &

dev-api: dev-init
	@echo "Starting complete api dev stack (docker)..."
	@$(DEV_COMPOSE) up -d --wait

dev-api-rebuild: dev-init
	@echo "Rebuilding and starting api service (docker)..."
	@$(DEV_COMPOSE) up -d --build --wait $(DEV_API_SERVICE)

dev-infra-up: dev-init
	@echo "Starting PostgreSQL and Redis for host backend development..."
	@$(DEV_COMPOSE) up -d --wait $(DEV_POSTGRES_SERVICE) redis

dev-infra-status: dev-init
	@$(DEV_COMPOSE) ps

dev-backend: dev-init
	@set -a; . ./$(DEV_ENV); set +a; go run .

dev-web: dev-init
	@set -a; . ./$(DEV_ENV); set +a; \
		port="$${DEV_FRONTEND_PORT:-$(DEV_WEB_DEFAULT_PORT)}"; \
		echo "Starting default web dev server..."; \
		echo "Default web: http://127.0.0.1:$$port"; \
		cd $(WEB_DIR) && $(BUN) run dev --host 127.0.0.1 --port "$$port"

dev-frontend: dev-web

dev-web-classic: dev-init
	@echo "Starting classic web dev server..."
	@cd $(WEB_CLASSIC_DIR) && $(BUN) run dev --host 127.0.0.1 --port $(DEV_WEB_CLASSIC_PORT)

dev: dev-api dev-web

dev-down: dev-init
	@$(DEV_COMPOSE) down

dev-reset: dev-init
	@$(DEV_COMPOSE) down --volumes --remove-orphans

# Optional developer-owned VPS mode. The standard local Compose workflow above
# remains the default for contributors without a personal VPS.
personal-dev-init:
	@$(PERSONAL_DEV_SCRIPT) init

personal-dev-up:
	@$(PERSONAL_DEV_SCRIPT) start

personal-dev-sync:
	@$(PERSONAL_DEV_SCRIPT) sync

personal-dev-rebuild:
	@$(PERSONAL_DEV_SCRIPT) rebuild

personal-dev-status:
	@$(PERSONAL_DEV_SCRIPT) status

personal-dev-logs:
	@$(PERSONAL_DEV_SCRIPT) logs

personal-dev-doctor:
	@$(PERSONAL_DEV_SCRIPT) doctor

personal-dev-down:
	@$(PERSONAL_DEV_SCRIPT) down

personal-dev-tunnel-up:
	@$(PERSONAL_DEV_SCRIPT) tunnel-up

personal-dev-tunnel-down:
	@$(PERSONAL_DEV_SCRIPT) tunnel-down

personal-dev-web-up:
	@$(PERSONAL_DEV_SCRIPT) web-up

personal-dev-web-status:
	@$(PERSONAL_DEV_SCRIPT) web-status

personal-dev-web-logs:
	@$(PERSONAL_DEV_SCRIPT) web-logs

personal-dev-web-down:
	@$(PERSONAL_DEV_SCRIPT) web-down

reset-setup: dev-init
	@echo "Resetting local setup wizard state..."
	@set -a; . ./$(DEV_ENV); set +a; \
	if $(DEV_COMPOSE) ps --services --status running | grep -qx "$(DEV_POSTGRES_SERVICE)"; then \
		echo "Detected running docker dev PostgreSQL. Removing setup record and root users..."; \
		$(DEV_COMPOSE) exec -T $(DEV_POSTGRES_SERVICE) \
			psql -U "$$DEV_POSTGRES_USER" -d "$$DEV_POSTGRES_DB" \
			-c 'DELETE FROM setups;' \
			-c 'DELETE FROM users WHERE role = 100;' \
			-c "DELETE FROM options WHERE key IN ('SelfUseModeEnabled', 'DemoSiteEnabled');"; \
		if $(DEV_COMPOSE) ps --services --status running | grep -qx "$(DEV_API_SERVICE)"; then \
			echo "Restarting docker dev api so setup status is recalculated..."; \
			$(DEV_COMPOSE) restart $(DEV_API_SERVICE); \
		else \
			echo "PostgreSQL setup state reset. Restart the host api process before testing."; \
		fi; \
	elif db_path="$${SQLITE_PATH:-$(DEV_SQLITE_PATH)}"; db_path="$${db_path%%\?*}"; [ -f "$$db_path" ]; then \
		db_path="$${SQLITE_PATH:-$(DEV_SQLITE_PATH)}"; \
		db_path="$${db_path%%\?*}"; \
		echo "Detected local SQLite database: $$db_path"; \
		sqlite3 "$$db_path" \
			"DELETE FROM setups; DELETE FROM users WHERE role = 100; DELETE FROM options WHERE key IN ('SelfUseModeEnabled', 'DemoSiteEnabled');"; \
		echo "SQLite setup state reset. Restart the local api process before testing the setup wizard."; \
	else \
		echo "No running docker dev PostgreSQL or local SQLite database found."; \
		echo "Start the dev stack with 'make dev-infra-up', or set SQLITE_PATH/DEV_SQLITE_PATH."; \
		exit 1; \
	fi
