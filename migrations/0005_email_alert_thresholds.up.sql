BEGIN;

ALTER TABLE email_alerts
ADD COLUMN IF NOT EXISTS min_threshold NUMERIC(20,8),
ADD COLUMN IF NOT EXISTS max_threshold NUMERIC(20,8),
ADD COLUMN IF NOT EXISTS is_active BOOLEAN NOT NULL DEFAULT true,
ADD COLUMN IF NOT EXISTS triggered_at TIMESTAMPTZ;

UPDATE email_alerts
SET min_threshold = threshold_value,
    max_threshold = threshold_value
WHERE min_threshold IS NULL
   OR max_threshold IS NULL;

COMMIT;
