-- Government Template Platform V3.0
-- 46_relay_webhook-ийг буцаана: чиглэл/webhook нууцыг хасаж, дээд демо peer-ийг устгана.

DELETE FROM relay_platforms WHERE code = 'e-mongolia';

ALTER TABLE relay_platforms DROP CONSTRAINT IF EXISTS relay_platforms_direction_chk;
ALTER TABLE relay_platforms
    DROP COLUMN IF EXISTS direction,
    DROP COLUMN IF EXISTS webhook_secret;
