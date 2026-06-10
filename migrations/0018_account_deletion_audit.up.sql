BEGIN;

-- Privacy-preserving deletion audit. When a user deletes their account, all personal data and the
-- encrypted Binance keys are still HARD-deleted via the ON DELETE CASCADE foreign keys (the right to
-- be forgotten is fully honored). This table keeps only a minimal, non-identifying record so abuse /
-- fraud can be investigated and deletions accounted for, WITHOUT storing any PII: the email is
-- reduced to a keyed one-way fingerprint (HMAC, not reversible without the server key) and the rest
-- is non-personal metadata. It has no foreign key to users, so it survives the cascade.
CREATE TABLE IF NOT EXISTS account_deletion_audit (
    id BIGSERIAL PRIMARY KEY,
    email_fingerprint TEXT,                -- HMAC-SHA256 of the lowercased email; NULL when no server key
    auth_method VARCHAR(20) NOT NULL,      -- password | google | both | unknown
    account_created_at TIMESTAMPTZ,
    had_binance_credentials BOOLEAN NOT NULL DEFAULT false,
    operation_count INTEGER NOT NULL DEFAULT 0,
    deleted_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS account_deletion_audit_fingerprint_idx ON account_deletion_audit (email_fingerprint);
CREATE INDEX IF NOT EXISTS account_deletion_audit_deleted_at_idx ON account_deletion_audit (deleted_at);

COMMIT;
