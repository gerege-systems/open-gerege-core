-- Government Template Platform V3.0
-- Landing (нүүр) хуудасны бүрэн theme систем. Нэг theme = харагдац (өнгөний
-- палетр · фонт · стиль · загвар) + landing-ийн бүх текст/цэс (mn/en). Олон
-- нэртэй theme үүсгэж, аль нэгийг нь ИДЭВХТЭЙ (default) болгоно — нэвтрээгүй
-- зочин идэвхтэй theme-ээр landing-ийг харна. Нэвтэрсэн апп үүнд хамаарахгүй.
--
-- config нь JSONB — уян хатан (frontend template default дээр deep-merge хийнэ):
--   { appearance: { mode, font, style, colors:{...base hex...} },
--     landing: { mn: LandingCopy, en: LandingCopy } }
-- Хоосон colors/landing нь "өөрчлөлтгүй" гэсэн үг (frontend template-ээ хэрэглэнэ).
--
-- Нийтийн config тул RLS-гүй; ГЭХДЭЭ ai_prompts-оос ялгаатай нь app бүрэн CRUD
-- хийдэг (админ theme үүсгэж/устгана, roles-той адил) тул grant хасахгүй.
CREATE TABLE IF NOT EXISTS themes (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT NOT NULL,
    config     JSONB NOT NULL DEFAULT '{}'::jsonb,
    is_active  BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ
);

-- Яг нэг идэвхтэй theme байхыг баталгаажуулна (partial unique index).
CREATE UNIQUE INDEX IF NOT EXISTS themes_one_active ON themes (is_active) WHERE is_active;

-- Анхдагч "DAN default" theme — хоосон colors/landing тул frontend одоогийн
-- харагдац/текстээ (globals.css + copy.ts) яг хэвээр үзүүлнэ (visual өөрчлөлтгүй).
INSERT INTO themes (name, config, is_active)
SELECT 'DAN default',
       '{"appearance":{"mode":"light","font":"inter","style":"comfortable","colors":{}},"landing":{"mn":{},"en":{}}}'::jsonb,
       true
WHERE NOT EXISTS (SELECT 1 FROM themes);
