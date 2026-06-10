-- Gerege Template Version 27.0
-- Хэрэглэгчийн овог (last_name) + нэр (first_name). Хоосон default тул одоо
-- байгаа мөрүүд эвдрэхгүй (backward compatible).

ALTER TABLE users ADD COLUMN IF NOT EXISTS first_name TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_name  TEXT NOT NULL DEFAULT '';
