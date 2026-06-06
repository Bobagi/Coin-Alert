# Coin Hub

A personal investing platform served at **https://coin.bobagi.space**. It unifies two
previously separate projects into one multi-user application:

- **Crypto** — log and automate Binance trades (market buy, take-profit limit sell, daily
  DCA, price alerts) with per-user, encrypted API credentials. Testnet by default; live
  trading is an explicit per-user opt-in.
- **B3 portfolio** — read an [Investidor10](https://investidor10.com.br) public wallet to
  show holdings, order history, and upcoming dividend (data-com) dates.

> Replaces the old `Bobagi/Coin-Alert` (Go) and `Bobagi/investidor10` (Python) repos, which
> are merged here.

## Architecture

```
coin.bobagi.space ──nginx(TLS)──▶ web (SvelteKit, Phase 4) ──▶ api (Go) ──▶ Postgres
                                                                   │
                                                                   └─▶ scraper (Python/Flask + Selenium)
```

| Path          | Service   | Stack                      | Role |
|---------------|-----------|----------------------------|------|
| `apps/api`    | `api`     | Go (SOLID, single binary)  | Trading engine + REST API + auth (core) |
| `apps/scraper`| `scraper` | Python 3.11 / Flask + Selenium | Scrapes Investidor10 wallets (internal-only service) |
| `apps/web`    | `web`     | SvelteKit (Phase 4)        | Dashboard SPA with real charts |
| `migrations`  | `migrate` | golang-migrate SQL         | Versioned DB schema |
| `deploy`      | —         | nginx vhost reference      | Ops reference |

## Local development

```bash
cp .env.example .env          # then fill in real values
docker compose up --build     # db + migrate + scraper + api
# API:    http://localhost:5020
# Scraper is internal-only (http://scraper:5000 from the API)
```

Apply a new migration: add `NNNN_name.up.sql` / `.down.sql` under `migrations/`, then
`docker compose up migrate`.

## Roadmap (tracked in the task list)

0. **Monorepo unification** — single repo, one compose. *(done)*
1. **Multi-user core** — users + auth, per-user encrypted Binance keys, user-scoped data.
2. **Trading hardening** — stop-loss + risk caps, WebSocket fills/price for fast reaction.
3. **Portfolio integration** — API calls the scraper; per-user wallet + caching.
4. **SvelteKit frontend** — auth UI, dashboards, and proper charts.
5. **Deploy** — ship to coin.bobagi.space, reset DB, decommission old containers/repo.

## Security posture

- Per-user Binance secrets are encrypted at rest (AES-256-GCM) and never logged.
- Use **trade-only** Binance API keys (withdrawals disabled).
- New users start on **Binance Testnet**; live trading requires explicit opt-in.
- Automated trading carries real financial risk; risk controls (stop-loss, caps) are built in.
