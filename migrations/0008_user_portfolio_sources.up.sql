BEGIN;

-- Per-user Investidor10 public wallet URL, used by the portfolio scraper.
CREATE TABLE IF NOT EXISTS user_portfolio_sources (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    investidor10_wallet_url TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMIT;
