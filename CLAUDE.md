# CLAUDE.md — Coin Hub

Guidance for Claude Code (and humans) working in this repo. Read this first; it is the source of
truth so you don't have to re-derive the project each session.

## What this is

**Coin Hub** is a multi-user personal investing app served at **https://coin.bobagi.space**. It
merges two former projects into one repo:
- **Crypto** (was `Bobagi/Coin-Alert`, Go): connect Binance, log/automate trades — market buy +
  take-profit limit sell, daily DCA, stop-loss, price alerts.
- **B3 portfolio** (was `Bobagi/investidor10`, Python): read an Investidor10 public wallet to show
  stocks/FIIs and upcoming ex-dividend (data-com) dates.

Owner: Gustavo Perin ("Bobagi"). Brand palette is **warm dark + gold** (`#ffd43b` / `#fab005`,
text `#fff9db`) to match his other sites; UI is trilingual (pt-BR/en/es, auto-detected).

## Repo layout (monorepo)

```
apps/api/      Go backend: trading engine + REST API + auth (the core). Module `coin-alert`.
apps/web/      Svelte + Vite SPA (TypeScript). Builds to apps/web/dist (served by nginx).
apps/scraper/  Python/Flask + Selenium scraper for Investidor10 (internal-only service).
migrations/    golang-migrate SQL (0001..NNNN), applied by the compose `migrate` service.
deploy/nginx/  Reference copy of the live vhost.
docker-compose.yml   db + migrate + api (+ scraper under the `scraper` profile).
.env           Real secrets (gitignored, chmod 600). Copy from .env.example.
```

### apps/api internals (Go, SOLID-ish layering)
`cmd/server/main.go` wires everything. `internal/`:
- `config` — env loading.  `database` — Postgres connector.  `domain` — structs/constants.
- `repository` — Postgres persistence; **everything is user-scoped** (`WHERE user_id = $1`).
- `service` — business logic: auth (bcrypt + sessions), `UserCredentialService` (per-user Binance
  keys, AES-256-GCM at rest), `UserTradingService` (buy = market + take-profit limit sell),
  `AutomationWorker` (per-user reconcile + stop-loss + daily DCA, 30s poll), Binance REST clients,
  `PortfolioScraperClient`.
- `httpserver` — JSON handlers: `auth_handler` (email + Google OAuth), `account_handler`
  (profile/password/delete), `api_handler` (settings/credentials/price/symbols),
  `operations_handler`, `portfolio_handler`. `server.go` is **legacy single-user dead code** (not
  wired; delete when convenient). Google OAuth lives in `service/google_oauth_service.go` (stdlib
  only, no extra module); it is **config-driven** — unset `GOOGLE_OAUTH_*` ⇒ feature off & button hidden.

## API surface (cookie-authenticated except signup/login/providers and the Google redirect flow)
`/auth/{signup,login,logout,me,providers}` · `/auth/google/{login,callback}` · `/api/v1/settings` (GET/PUT) ·
`/api/v1/binance/{credentials,credentials/activate,price,symbols,open-orders}` ·
`/api/v1/operations` (GET list / POST buy) · `/api/v1/operations/sell` (POST close-now at market) · `/api/v1/operations/executions` ·
`/api/v1/portfolio/{source,assets,dividends}` ·
`/api/v1/account/profile` (PUT) · `/api/v1/account/password` (POST) · `/api/v1/account` (DELETE) · `/health`.
Sessions = opaque random token in a Secure httpOnly cookie (`coin_hub_session`); only its SHA-256
hash is stored.

## Build & run (IMPORTANT gotchas)

- **Go is NOT in PATH.** Build/test via Docker:
  `docker run --rm -v "$PWD":/app -w /app -e GOTOOLCHAIN=local golang:1.22-alpine sh -c "go build ./... && go vet ./..."`
  (run from `apps/api`). `golang.org/x/crypto` is **pinned to v0.31.0** (newer needs Go ≥1.25).
- **Frontend:** Node 18 + pnpm 9 via nvm. `cd apps/web && export PATH="$HOME/.nvm/versions/node/v18.20.5/bin:$PATH" && pnpm install && pnpm build`. nginx serves `dist/` directly, so after `pnpm build` the new UI is live (no container/nginx reload needed). `package.json` has `pnpm.onlyBuiltDependencies:["esbuild"]` so the build script runs.
- **Edit `.svelte` source lives in `apps/web/src/lib/`** — the repo-root `.gitignore` ignores `lib/`,
  so `apps/web/.gitignore` re-includes it (`!src/lib/`). Don't remove that or the UI source stops
  being committed.

## Deploy (production, on the VPS)

```bash
cp .env.example .env   # first time; fill DB_PASSWORD + CREDENTIALS_ENCRYPTION_KEY (openssl rand -base64 32)
docker compose up -d --build                    # db + migrate + api
docker compose --profile scraper up -d --build  # also build/start the scraper
cd apps/web && pnpm build                        # rebuild the SPA nginx serves
```
- Compose project name **`coin-hub`**: `coin-hub-db-1`, `coin-hub-api-1`, `coin-hub-scraper-1`
  (all `restart: always`). API listens on **127.0.0.1:5020** only; nginx fronts it.
- DB is **internal-only** (no host port). Volume `coin-hub_db_data`.
- nginx vhost: `/etc/nginx/sites-available/coin.bobagi.space` (TLS via certbot) serves
  `/opt/Coin-Alert/apps/web/dist` and proxies `/api`,`/auth`,`/health` → :5020. After edits:
  `nginx -t && systemctl reload nginx`.
- **`CREDENTIALS_ENCRYPTION_KEY` must stay stable** — regenerating it makes stored Binance secrets
  undecryptable. Never print/commit `.env`.
- `apps/api` runs on **distroless** (no shell): debug via `docker logs coin-hub-api-1`, not `exec`.

## Conventions
- Descriptive English identifiers (functions/vars), even when chatting in PT.
- Migrations are **additive** and versioned; the app enforces user scoping in code.
- **Testnet-first**: new users default to TESTNET; live (PRODUCTION) orders are refused unless the
  user set `live_trading_enabled`. Recommend trade-only Binance keys (no withdrawal).
- i18n: `apps/web/src/lib/i18n.ts` (dictionaries en/pt/es + `t` store + auto-detect). Add UI strings
  there, not inline.

## Status (2026-06)
Done & live: monorepo unification; multi-user auth (email + **Google OAuth**, migration 0009 makes
`password_hash` nullable + adds `google_subject`); **account settings page** (edit name, set/change
password, language, delete account — cascades via the user FKs); per-user encrypted Binance creds;
settings (incl. **daily-buy on/off toggle** `daily_purchase_enabled`, migration 0010); operations
(manual buy + take-profit + **manual close-now** `CloseOperationNow`); **per-environment isolation**
(migration 0011: `binance_environment` tags operations/executions and is part of the
`user_trading_settings` composite PK — listings, the worker and settings all scope to the user's active
environment via `UserCredentialService.ActiveEnvironmentName`); automation worker (reconcile + stop-loss
+ daily DCA, skipped when the toggle is off); Svelte SPA with a **design system** (rem type scale +
spacing tokens in `app.css`, sticky `TopNav`, **SVG flags** in `LanguageDropdown` — emoji flags break on
Windows, hash router in `stores.ts`), a **3-tab dashboard** (Binance connection [default] / Trade /
B3-Investidor10) with an **environment switcher** (buttons; selecting activates + reloads) + **symbol
autocomplete** (`SymbolAutocomplete`, via `/binance/symbols`), a **bot-status panel** with an on/off
button + **local-timezone** daily-buy picker, allocation chart with value/% legend, an **operations
history sub-tab** (executions, for auditing), a **non-custodial disclaimer/ToS** (`LegalFooter`),
explanations, gold theme, favicon, i18n; portfolio scraper integration. Pending/optional: per-user email price alerts (table
exists, route not rebuilt); more
chart types (PnL/price/dividend calendar); WebSocket fills/price (today 30s polling; take-profit is
already a resting limit order at exchange speed); delete legacy `server.go` + templates; decommission
the old standalone `investidor10` container (:3054), now redundant.

## Don't print secrets
`.env`, `/root/commands_band_share.txt`, and any API keys. Never echo/commit them.
