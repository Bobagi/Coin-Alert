# Coin Alert (Go)

Web dashboard to log cryptocurrency trades, validate Binance API credentials, and send email alerts, rewritten in Go following SOLID boundaries.

## Overview
- API and dashboard served by a single Go binary.
- PostgreSQL stores trades, email alerts, and Binance credentials.
- Internal automation services for scheduled buy/sell intervals.
- Docker Compose with one application container and one PostgreSQL container.

## Environment variables
Copy `.env.example` to `.env` and adjust the values to match your environment (database credentials, SMTP, Binance keys, and scheduler intervals).

## Running with Docker
1. Build and start the containers:
   ```
   docker compose up --build
   ```
2. Open `http://localhost:${API_PORT:-5020}` (or the port configured in `API_PORT`) to access the dashboard.

Database migrations run in the separate `migrate` service before the application starts, keeping schema creation and evolution out of the application runtime.

### Updating database credentials after data already exists
If you change `DB_USER` or `DB_PASSWORD` after the `db_data` volume already exists, PostgreSQL will keep the original credentials stored in the volume. To apply new credentials, remove the volume before starting again:

```
docker compose down -v
docker compose up --build
```

Alternatively, keep the same credentials used during the first database initialization.

## Project structure
- `cmd/server`: application entrypoint.
- `internal/config`: environment configuration loading.
- `internal/database`: PostgreSQL connector and connection lifecycle.
- `internal/domain`: domain models.
- `internal/repository`: PostgreSQL persistence.
- `internal/service`: business rules and automation services.
- `internal/httpserver`: HTTP handlers and template rendering.
- `migrations`: versioned database migrations executed by the `migrate` service.
- `templates`: HTML/CSS dashboard.

## Features
- Unified trading operations: purchase, monitor profit target, and mark as sold in a single log.
- Capital threshold enforcement for the configured trading pair with automatic sell monitoring.
- List recent operations with purchase and sell details.
- Send authenticated SMTP email alerts with persistence.
- Scheduled operations persisted for visibility, including the next predicted action and manual "execute now" trigger.
- Execution history recorded for every automated attempt with success and error details.
