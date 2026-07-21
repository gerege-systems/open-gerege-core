-- Government Template Platform V3.0
-- Хэрэглэгчийн англи (Латин) овог/нэр — хэл солиход нэрийг тухайн хэлээр
-- үзүүлэхэд. Хоосон default тул backward compatible.

ALTER TABLE users ADD COLUMN IF NOT EXISTS first_name_en TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS last_name_en  TEXT NOT NULL DEFAULT '';
