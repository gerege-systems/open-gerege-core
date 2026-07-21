-- Government Template Platform V3.0
-- Relay-г хоёр чиглэлтэй болгоно: platform бүр upstream (дээд) эсвэл downstream
-- (доод) гэсэн чиглэлтэй, webhook гарын үсэг зурах нууц түлхүүртэй болно. Ингэснээр
-- дээшээ/доошоо хүсэлт webhook-оор дамжуулж, ирсэн webhook-ийг гарын үсгээр
-- баталгаажуулна.

ALTER TABLE relay_platforms
    ADD COLUMN IF NOT EXISTS direction      TEXT NOT NULL DEFAULT 'downstream',
    ADD COLUMN IF NOT EXISTS webhook_secret TEXT NOT NULL DEFAULT '';

-- Чиглэл нь зөвхөн upstream|downstream байна.
ALTER TABLE relay_platforms DROP CONSTRAINT IF EXISTS relay_platforms_direction_chk;
ALTER TABLE relay_platforms
    ADD CONSTRAINT relay_platforms_direction_chk CHECK (direction IN ('upstream', 'downstream'));

-- Байгаа демо platform-ууд доод (downstream) хэвээр (default). Дээд (upstream)
-- демо peer нэмнэ — И-Монгол нь дээрээс хүсэлт илгээж, бид түүнд хариу дамжуулна.
INSERT INTO relay_platforms (code, name, endpoint_url, supervisor_contact, direction, webhook_secret)
SELECT 'e-mongolia', 'И-Монгол (дээд платформ)', 'demo://loopback', 'supervisor@e-mongolia.mn', 'upstream',
       replace(gen_random_uuid()::text, '-', '') || replace(gen_random_uuid()::text, '-', '')
WHERE NOT EXISTS (SELECT 1 FROM relay_platforms WHERE code = 'e-mongolia');

-- Нууц түлхүүргүй хуучин мөрүүдэд санамсаргүй webhook_secret оноох (64 hex тэмдэгт).
UPDATE relay_platforms
SET webhook_secret = replace(gen_random_uuid()::text, '-', '') || replace(gen_random_uuid()::text, '-', '')
WHERE webhook_secret = '';
