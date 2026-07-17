-- Gerege Template Version 27.0
-- Нүүр хуудасны (landing) ажиллаж байх үед тохируулдаг харагдац: өнгө, фонт,
-- хэмжээ, текст (mn/en), товч/цэс. ai_prompts-той адил хэрэглэгч-тус-бүрийн
-- биш ГЛОБАЛ тохиргоо тул Row-Level Security-гүй; ганц мөр (id=1) байнга
-- байх ба апп зөвхөн UPDATE хийнэ (шинэ мөр нэмэхгүй/устгахгүй).
--
-- config нь нэг JSON баримт — схемийг frontend эзэмшдэг (lib/landing.ts);
-- backend үүнийг зөвхөн хүчинтэй JSON объект бөгөөд хэмжээний хязгаарт
-- багтаж буйг шалгаж, rawCss талбарыг ариутгаад хадгална. Анхны seed нь
-- одоогийн нүүрний агуулгыг (текст mn+en, eID/SSO товч, badge) агуулах тул
-- админ гар хүрээгүй үед хуудас яг одоогийнхтой адил харагдана; theme
-- талбарууд хоосон = globals.css-ийн өгөгдмөл (light/dark аль аль нь хэвээр).
CREATE TABLE IF NOT EXISTS landing_config (
    id         INT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    config     JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ
);

INSERT INTO landing_config (id, config) VALUES (1, $json${
  "theme": {
    "colors": { "danBlue": "", "danBlueHover": "", "gold": "", "bg": "", "surface": "", "fg": "", "border": "" },
    "fonts":  { "displayStack": "", "bodyStack": "", "monoStack": "" },
    "sizes":  { "titlePx": "", "bodyPx": "", "radiusCard": "", "radiusInput": "", "topbarH": "" },
    "weights":{ "title": "" }
  },
  "rawCss": "",
  "brand":  { "name": { "mn": "Gerege Template", "en": "Gerege Template" }, "logoUrl": "/brand.webp" },
  "nav": [],
  "content": {
    "title": { "mn": "Gerege Template", "en": "Gerege Template" },
    "lede":  {
      "mn": "Gerege Template (chi + pgx) дээр суурилсан жишээ апп. eID апп-аараа QR кодыг уншуулан нэвтэрч, профайл болон аюулгүй байдлын тохиргоогоо нэг дороос удирдана.",
      "en": "A sample app built on Gerege Template (chi + pgx). Scan the QR code with your eID app to sign in and manage your profile and security settings in one place."
    },
    "helper": {
      "mn": "Нэвтрэлт нь eID аппаар баталгаажиж, богино TTL-тэй access болон урт TTL-тэй refresh JWT хослолоор хийгдэнэ.",
      "en": "Sign-in is verified by the eID app and uses a short-TTL access plus long-TTL refresh JWT pair."
    }
  },
  "buttons": [
    { "id": "eid", "label": { "mn": "eID-ээр нэвтрэх", "en": "Sign in with eID" }, "action": "/login", "variant": "primary", "icon": "LogIn", "show": true, "order": 1 },
    { "id": "sso", "label": { "mn": "Gerege SSO-гоор нэвтрэх", "en": "Sign in with Gerege SSO" }, "action": "/api/auth/sso/start", "variant": "secondary", "icon": "ShieldCheck", "show": true, "order": 2 }
  ],
  "badges": [
    { "label": { "mn": "JWT", "en": "JWT" }, "show": true },
    { "label": { "mn": "eID", "en": "eID" }, "show": true },
    { "label": { "mn": "chi + pgx", "en": "chi + pgx" }, "show": true },
    { "label": { "mn": "TLS 1.3", "en": "TLS 1.3" }, "show": true }
  ],
  "footer": { "text": { "mn": "© 2026 Gerege Systems · Gerege Template", "en": "© 2026 Gerege Systems · Gerege Template" } }
}$json$::jsonb)
ON CONFLICT (id) DO NOTHING;

-- Defense-in-depth (migration 17-той адил): апп-ын урсгал зөвхөн UPDATE тул
-- app_user-аас INSERT/DELETE-ийг хасна — authz шалгалт алдагдсан ч тохиргооны
-- мөрийг нэмэх/устгах боломжгүй. app_user биш rolename дээр no-op (гараар
-- мирроорлоно).
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_user') THEN
        RAISE NOTICE 'app_user role not found — skipping landing_config privilege tightening (custom APP_DB_USER? mirror these REVOKEs by hand)';
        RETURN;
    END IF;
    REVOKE INSERT, DELETE ON landing_config FROM app_user;
END $$;
