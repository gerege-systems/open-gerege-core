-- Динамик хэлний систем. Хүртэл frontend-ийн dictionary нь compile-time байсан
-- тул шинэ хэл нэмэх бүрд deploy шаарддаг байв. Одоо super admin нь хэлийг
-- ажиллаж байх үед нэмж/хасч, орчуулгыг нь (гараар эсвэл Gemini-ээр) бөглөнө.
--
-- ЗАРЧИМ: түлхүүрийн ЖАГСААЛТ нь аппын өөрийнх (frontend-ийн багцлагдсан
-- dictionary) хэвээр — платформ нь тэдгээрийн УТГЫГ л хадгална. Ингэснээр
-- platform-core нь тухайн аппын түлхүүрийг мэдэхгүйгээр ерөнхий хэвээр үлдэнэ,
-- мөн DB унасан ч апп багцлагдсан утгаараа ажилласаар байна.
--
-- Нийтийн config тул RLS-гүй (themes-тэй ижил зарчим): унших нь нээлттэй,
-- бичих нь зөвхөн super admin (RequireSuperAdmin route gate).
CREATE TABLE IF NOT EXISTS languages (
    -- BCP-47 хэлний код: 'mn', 'en', 'zh', 'ja', 'zh-Hans' г.м.
    code       TEXT PRIMARY KEY,
    -- Хэлний ЭХ нэр (сонгогчид харагдана): 'Монгол', '日本語'.
    label      TEXT NOT NULL,
    -- Intl (огноо/тоо) форматлах locale: 'mn-MN', 'ja-JP'.
    locale     TEXT NOT NULL,
    -- Хэрэглэгчид харагдах эсэх. Шинэ хэл нь орчуулга бөглөгдтөл унтраалттай.
    enabled    BOOLEAN NOT NULL DEFAULT false,
    -- Аппын кодод багцлагдсан хэл — устгаж БОЛОХГҮЙ (зөвхөн унтраана), учир нь
    -- эдгээрийн утга DB-гүйгээр ч frontend-д байдаг (fallback).
    is_builtin BOOLEAN NOT NULL DEFAULT false,
    sort_order INTEGER NOT NULL DEFAULT 100,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ
);

-- Нэг хэлний бүх мөр = тухайн хэлний dictionary. Түлхүүр нь аппынх тул энд
-- гадаад түлхүүр байхгүй — зөвхөн текст.
CREATE TABLE IF NOT EXISTS translations (
    lang_code  TEXT NOT NULL REFERENCES languages(code) ON UPDATE CASCADE ON DELETE CASCADE,
    key        TEXT NOT NULL,
    value      TEXT NOT NULL,
    -- Утга хаанаас ирсэн: 'manual' (гараар), 'ai' (Gemini), 'import' (JSON).
    -- AI-аар үүсгэснийг дараа нь гараар засвал 'manual' болно — дахин
    -- автомат орчуулга хийхэд гар засварыг дарж бичихгүй.
    source     TEXT NOT NULL DEFAULT 'manual',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (lang_code, key)
);

-- Багцлагдсан дөрвөн хэл. Утга нь frontend-ийн dictionary-д байгаа тул энд
-- translations мөр оруулахгүй — DB хоосон байхад апп багцлагдсанаараа ажиллана.
INSERT INTO languages (code, label, locale, enabled, is_builtin, sort_order) VALUES
    ('mn', 'Монгол',  'mn-MN', true, true, 10),
    ('en', 'English', 'en-US', true, true, 20),
    ('zh', '中文',     'zh-CN', true, true, 30),
    ('ru', 'Русский', 'ru-RU', true, true, 40)
ON CONFLICT (code) DO NOTHING;
