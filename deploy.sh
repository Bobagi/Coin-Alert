#!/usr/bin/env bash
# Deploy helper for Coin Hub (https://coin.bobagi.space).
# Goal: one command, concise output — so a deploy is one tool call / one log to read.
#
# Usage:
#   ./deploy.sh            # web (default): rebuild the SPA nginx serves — live immediately
#   ./deploy.sh api        # rebuild + restart db/migrate/api containers
#   ./deploy.sh scraper    # rebuild + restart the scraper (profile)
#   ./deploy.sh all        # web + api + scraper
#   ./deploy.sh web api     # any combination of: web api scraper all
#
# Notes:
# - The frontend builds to apps/web/dist, which nginx serves directly: pnpm build IS the deploy.
# - Git commit is intentionally NOT done here (a deploy shouldn't auto-commit with a junk message);
#   commit separately with a real message.
set -euo pipefail
cd "$(dirname "$0")"

NODE_BIN="$HOME/.nvm/versions/node/v18.20.5/bin"
COMPOSE="docker compose"

step() { printf '\n\033[1;33m▶ %s\033[0m\n' "$*"; }
ok()   { printf '\033[1;32m✓ %s\033[0m\n' "$*"; }
die()  { printf '\033[1;31m✗ %s\033[0m\n' "$*" >&2; exit 1; }

deploy_web() {
  step "Building SPA (apps/web)"
  command -v pnpm >/dev/null 2>&1 || export PATH="$NODE_BIN:$PATH"
  ( cd apps/web && pnpm install --silent && pnpm build ) || die "web build failed"
  ok "SPA built → apps/web/dist (live via nginx)"
}

deploy_api() {
  step "Building + starting db/migrate/api"
  $COMPOSE up -d --build db migrate api || die "api compose failed"
  ok "api up"
}

deploy_scraper() {
  step "Building + starting scraper"
  $COMPOSE --profile scraper up -d --build scraper || die "scraper compose failed"
  ok "scraper up"
}

health() {
  step "Health check"
  local code
  code=$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:5020/health || echo 000)
  [ "$code" = "200" ] && ok "API /health → 200" || printf '\033[1;31m✗ API /health → %s\033[0m\n' "$code"
}

targets=("${@:-web}")
do_web=0 do_api=0 do_scraper=0
for t in "${targets[@]}"; do
  case "$t" in
    web) do_web=1 ;;
    api) do_api=1 ;;
    scraper) do_scraper=1 ;;
    all) do_web=1; do_api=1; do_scraper=1 ;;
    *) die "unknown target '$t' (use: web api scraper all)" ;;
  esac
done

[ "$do_web" = 1 ] && deploy_web
[ "$do_api" = 1 ] && deploy_api
[ "$do_scraper" = 1 ] && deploy_scraper
[ "$do_api" = 1 ] && health

printf '\n\033[1;32m✓ Deploy done.\033[0m\n'
