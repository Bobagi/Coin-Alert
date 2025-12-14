BEGIN;

DROP TABLE IF EXISTS trading_operation_executions;
DROP TABLE IF EXISTS scheduled_trading_operations;
DROP TABLE IF EXISTS email_alerts;
CREATE TABLE email_alerts (
    id SERIAL PRIMARY KEY,
    recipient_address VARCHAR(255) NOT NULL,
    subject TEXT NOT NULL,
    message_body TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

COMMIT;
