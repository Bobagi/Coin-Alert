BEGIN;

-- Admin role. Admins get access to the B3/Investidor10 tab and unlimited trading robots;
-- standard users are limited (monetization hook). Defaults to false for everyone.
ALTER TABLE users ADD COLUMN IF NOT EXISTS is_admin BOOLEAN NOT NULL DEFAULT false;

-- Seed the owner account as admin.
UPDATE users SET is_admin = true WHERE LOWER(email) = LOWER('gustavoperin067@gmail.com');

COMMIT;
