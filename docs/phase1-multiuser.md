# Phase 1 — Multi-user core (design)

## Goal
Turn the single-tenant Go app (one global Binance credential + singleton background jobs)
into a multi-user platform where each user has their own credentials, settings, trades, and
automation — Testnet by default, live trading by explicit opt-in.

## Data model (migrations 0006–0007)
- `users` — email + bcrypt `password_hash`, `display_name`, `is_active`.
- `user_sessions` — server-side sessions; only `session_token_hash` is stored. Auth is a
  secure, httpOnly, SameSite cookie carrying an opaque random token.
- `user_trading_settings` — per-user `trading_pair_symbol`, `capital_threshold`,
  `target_profit_percent`, `stop_loss_percent`, intervals, `live_trading_enabled`,
  `active_binance_environment`. Replaces the old env/in-memory globals.
- `user_id` added to `binance_credentials`, `trading_operations`,
  `scheduled_trading_operations`, `trading_operation_executions`,
  `daily_purchase_settings`, `email_alerts`.

## Security
- **Passwords**: bcrypt (cost 12).
- **Sessions**: 256-bit random token; store SHA-256 hash; cookie `Secure; HttpOnly; SameSite=Lax`.
- **Binance secret at rest**: AES-256-GCM using `CREDENTIALS_ENCRYPTION_KEY` (base64 32 bytes).
  API secret (and key) are encrypted before insert and decrypted only in memory at trade time.
  Never logged. Recommend trade-only keys (withdrawals disabled).

## Service refactor (in progress)
- `CredentialService` becomes per-user: load/decrypt a given user's active credential on demand
  instead of holding one global key in a struct field.
- Repositories take `userID` and add `WHERE user_id = $1` to every query.
- HTTP handlers resolve the current user from the session cookie (auth middleware) and pass
  `userID` down. The dashboard no longer requires a single global credential to render.
- Background automation iterates over active users:
  - auto-sell / reconciliation loop: per user with valid credentials,
  - daily DCA: per user at their configured hour,
  - email-alert monitoring: per user's alert definitions.
  (Phase 2 replaces polling with Binance WebSocket streams for fast reaction.)

## Rollout
`user_id` ships nullable to keep the migration non-destructive. The Phase 5 deploy starts from
a fresh DB (user-authorized reset), after which `user_id` is tightened to NOT NULL.
