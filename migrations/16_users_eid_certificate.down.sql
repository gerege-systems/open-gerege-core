-- Gerege Template Version 27.0
-- 16_users_eid_certificate.up.sql-ийг буцаана.

ALTER TABLE users DROP COLUMN IF EXISTS document_number;
ALTER TABLE users DROP COLUMN IF EXISTS cert_serial;
ALTER TABLE users DROP COLUMN IF EXISTS cert_not_before;
ALTER TABLE users DROP COLUMN IF EXISTS cert_not_after;
ALTER TABLE users DROP COLUMN IF EXISTS cert_issuer;
ALTER TABLE users DROP COLUMN IF EXISTS cert_key_type;
