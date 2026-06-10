BEGIN;

-- One-time tokens for password reset and email verification. Only the SHA-256 hash of the token is
-- stored (the raw token is emailed to the user), the same way sessions are stored.
CREATE TABLE IF NOT EXISTS auth_tokens (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose VARCHAR(32) NOT NULL,          -- 'password_reset' | 'email_verification'
    token_hash TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX IF NOT EXISTS auth_tokens_token_hash_unique ON auth_tokens (token_hash);
CREATE INDEX IF NOT EXISTS auth_tokens_user_purpose_idx ON auth_tokens (user_id, purpose);
CREATE INDEX IF NOT EXISTS auth_tokens_expires_at_idx ON auth_tokens (expires_at);

-- Email verification state. Existing accounts are grandfathered in as verified so the new flow does
-- not retroactively lock anyone out; Google sign-ups are marked verified at creation time in code.
ALTER TABLE users ADD COLUMN IF NOT EXISTS email_verified_at TIMESTAMPTZ;
UPDATE users SET email_verified_at = COALESCE(created_at, NOW()) WHERE email_verified_at IS NULL;

COMMIT;
