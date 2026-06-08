BEGIN;

-- Google OAuth sign-in. Users can now register/login with Google, so a local password becomes
-- optional, and we store the Google account's stable subject identifier (the OIDC `sub` claim).
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;
ALTER TABLE users ADD COLUMN IF NOT EXISTS google_subject VARCHAR(255);

-- One Google identity maps to at most one account. Partial unique index so the many NULLs
-- (email/password-only accounts) do not collide.
CREATE UNIQUE INDEX IF NOT EXISTS users_google_subject_unique
    ON users (google_subject) WHERE google_subject IS NOT NULL;

COMMIT;
