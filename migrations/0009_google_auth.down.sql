BEGIN;

DROP INDEX IF EXISTS users_google_subject_unique;
ALTER TABLE users DROP COLUMN IF EXISTS google_subject;
-- Best-effort restore of the original constraint (fails if any passwordless Google account remains).
ALTER TABLE users ALTER COLUMN password_hash SET NOT NULL;

COMMIT;
