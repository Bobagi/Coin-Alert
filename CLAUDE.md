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
  `operations_handler`, `portfolio_handler`. (The legacy single-user `server.go` + HTML `templates/`
  were removed in the 2026-06 hardening pass; the legacy single-user *services* it used remain but
  are unwired/dead.) Google OAuth lives in `service/google_oauth_service.go` (stdlib
  only, no extra module); it is **config-driven** — unset `GOOGLE_OAUTH_*` ⇒ feature off & button hidden.

## API surface (cookie-authenticated except signup/login/providers and the Google redirect flow)
`/auth/{signup,login,logout,me,providers}` · `/auth/google/{login,callback}` · `/api/v1/settings` (GET/PUT) ·
`/api/v1/binance/{credentials,credentials/activate,price,symbols,symbol-filters,klines,open-orders}` ·
`/api/v1/operations` (GET list / POST buy) · `/api/v1/operations/sell` (POST close-now) · `/api/v1/operations/place-sell` (POST (re)place take-profit) · `/api/v1/operations/executions` ·
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
(manual buy + take-profit + **manual close-now** `CloseOperationNow` + **(re)place take-profit**
`PlaceTakeProfitForOperation`; orders snap price→tickSize, qty→stepSize and pre-check minNotional via
`FetchSymbolFilters`, so -1013 PRICE_FILTER/NOTIONAL become clear messages); **per-environment isolation**
(migration 0011: `binance_environment` tags operations/executions and is part of the
`user_trading_settings` composite PK — listings, the worker and settings all scope to the user's active
environment via `UserCredentialService.ActiveEnvironmentName`); automation worker (reconcile + stop-loss
+ daily DCA, skipped when the toggle is off; also **detects external take-profit cancellation**
(Binance status CANCELED → operation CANCELED/released, dropped from Positions) and enforces an
**app-side sell-order validity** — migration 0013 `sell_order_validity_days` (0=GTC) +
`sell_order_expires_at`; on expiry it cancels the order and leaves the position ⚠ to re-place/sell);
only **successful** executions are logged to history (failed attempts surface live + as ⚠, not as
0/0/0 rows); Svelte SPA with a **design system** (rem type scale +
spacing tokens in `app.css`, sticky `TopNav`, **SVG flags** in `LanguageDropdown` — emoji flags break on
Windows, hash router in `stores.ts`), a **3-tab dashboard** (Binance connection [default] / Trade /
B3-Investidor10) with an **environment switcher** (buttons; selecting activates + reloads) + **symbol
autocomplete** (`SymbolAutocomplete`, via `/binance/symbols`), a **bot-status panel** with an on/off
button + **local-timezone** daily-buy picker, a rich **`AllocationPanel`** (wallet donut by current
value + total + legend on the left; selected-coin header with period change badge, value, a
price-history line chart, 24h/7d/1M/3M tabs and coin pills on the right — holdings × current price,
history via `/binance/klines`), an **operations history sub-tab** (executions, for auditing — with a **By** column showing who acted,
`initiated_by` USER/BOT, migration 0012; the take-profit is GTC/no-expiry, shown in the Sell-order column), a **non-custodial disclaimer/ToS** (`LegalFooter`),
explanations, gold theme, favicon, i18n; portfolio scraper integration. Pending/optional: per-user email price alerts (table
exists, route not rebuilt); more
chart types (PnL/price/dividend calendar); WebSocket fills/price (today 30s polling; take-profit is
already a resting limit order at exchange speed); remove the now-unwired legacy single-user *services*
(`server.go` + `templates/` already deleted). The old standalone `investidor10` container (:3054) +
its `investidor10.bobagi.space` vhost were **decommissioned** in the 2026-06 hardening pass (compose
project at `/opt/investidor10` left on disk + the vhost kept in `sites-available`, so it is reversible).

### 2026-06 robots + admin (multi-bot model)
- **Admin role** (migration 0014 `users.is_admin`, owner seeded): admins access the **B3 tab** (the
  whole portfolio API is now admin-gated, 403 otherwise) and get unlimited robots. Exposed on `/auth/me`.
- **Trading robots** (migration 0015 `trading_robots`): a "robot" is one automated bot per coin/pair
  per environment (`/api/v1/robots` GET/POST · `/robots/update` · `/robots/delete`,
  `service.RobotService`). It **replaced the single per-environment automation**: the `AutomationWorker`
  now iterates robots (per-coin daily DCA with **per-symbol** idempotency, per-robot stop-loss) instead
  of one `user_trading_settings` row. Existing settings rows were seeded into one robot each, preserving
  behavior. `user_trading_settings` now only carries account-level bits used by the worker’s gate
  (`live_trading_enabled`) + manual-buy defaults; its other fields are no longer read by automation.
  **Standard users: 1 robot per environment; admins unlimited** (`StandardUserRobotLimitPerEnvironment`,
  monetization hook — payment not built yet).
- UI: the **Trade tab is first/default**; the bot-settings panel is hidden behind a **Robots list**
  (create → click to edit). History now distinguishes a placed take-profit (`SELL_ORDER_PLACED`, blue)
  from a completed sale (`SELL`/“Sold”, green). **Google sign-in is live** (`GOOGLE_OAUTH_*` set in
  `.env`); the OAuth consent screen is in *Testing*, so test users must be allow-listed in Google Cloud.
  `AuthenticateWithGoogle` auto-links by verified email (a manual account keeps its password).

### Lock UI + environment guarantees (2026-06)
- The **Trade** tab is **locked** (padlock left of the name, dimmed content + `LockOverlay` alert) until
  the user has a configured Binance environment — the Connection tab stays open (it's where you set it
  up). The **B3** tab is shown to everyone but **locked for non-admins** ("under construction"), not hidden.
- DB enforces an environment on every trade: migration 0017 makes `binance_environment` NOT NULL +
  CHECK `IN ('TESTNET','PRODUCTION')` on `trading_operations` and `trading_operation_executions`.
- **Account deletion is a privacy-preserving hard delete** (migration 0018): all PII + encrypted keys are
  erased via cascade, but one non-identifying row is written to `account_deletion_audit` — a keyed
  **HMAC email fingerprint** (irreversible without the server key, never the raw email) + `auth_method`,
  `account_created_at`, `had_binance_credentials`, `operation_count`. Best-effort, never blocks deletion.

## Don't print secrets
`.env`, `/root/commands_band_share.txt`, and any API keys. Never echo/commit them.
