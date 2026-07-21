-- Government Template Platform V3.0
-- Ring System · R1 — Үйлчилгээний нэгдсэн регистр (Service Registry).
--
-- Төрийн бүх үйлчилгээний МАСТЕР ӨГӨГДӨЛ: CPSV-AP-д нийцсэн "үйлчилгээний
-- паспорт", түүний хувилбарын түүх (baseline-тай харьцуулсан delta),
-- нотолгооны (бичиг баримтын) каталог ба аль нь ХУР-д аль хэдийн байгаа
-- вэ гэсэн mapping, амьдралын/бизнесийн үйл явдлын давхарга.
--
-- RLS: эдгээр нь хэрэглэгч-тус-бүрийн БИШ, байгууллагын нийтлэг мастер
-- өгөгдөл тул (gateway/relay хүснэгтүүдтэй ижил ангилал) Row-Level Security
-- хэрэглэхгүй. Хамгаалалт нь HTTP давхаргад: бичих/удирдах endpoint бүр
-- 'registry.manage' эрх шаардана (route_registry.go), нийтийн каталог нь
-- зөвхөн status='published' мөрүүдийг харуулна.
--
-- Хүснэгтэд DML эрх нь initdb-ийн ALTER DEFAULT PRIVILEGES-ээр app role-д
-- автоматаар олгогдоно (migration 20-той ижил загвар).

-- ── Амьдралын / бизнесийн үйл явдал (Сингапур LifeSG, БНСУ Government24) ───

CREATE TABLE IF NOT EXISTS registry_life_events (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    code        TEXT UNIQUE NOT NULL,
    name        TEXT NOT NULL,
    kind        TEXT NOT NULL DEFAULT 'life',   -- life | business
    description TEXT NOT NULL DEFAULT '',
    lead_agency TEXT NOT NULL DEFAULT '',       -- тэргүүлэх агентлаг
    sort_order  INT  NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT registry_life_events_kind_chk CHECK (kind IN ('life', 'business'))
);

-- ── Нотолгооны каталог (Evidence) + ХУР mapping (Эстони once-only) ─────────
-- in_khur = энэ бичиг баримтын мэдээлэл ХУР-д (эсвэл өөр төрийн санд) аль
-- хэдийн байгаа эсэх. Хэрэв байгаа атал үйлчилгээ түүнийг ИРГЭНЭЭС шаардаж
-- байвал энэ нь once-only зөрчил (registry_once_only_violations view).

CREATE TABLE IF NOT EXISTS registry_evidences (
    id                uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    code              TEXT UNIQUE NOT NULL,
    name              TEXT NOT NULL,
    description       TEXT NOT NULL DEFAULT '',
    holder_agency     TEXT NOT NULL DEFAULT '',   -- эзэмшигч байгууллага
    source_system     TEXT NOT NULL DEFAULT '',   -- эх мэдээллийн систем
    in_khur           BOOLEAN NOT NULL DEFAULT false,
    khur_service_code TEXT NOT NULL DEFAULT '',   -- ХУР-ын лавлагааны код
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ
);

-- ── Үйлчилгээний паспорт (CPSV-AP + Казахстаны "стандарт") ────────────────
-- Талбарууд CPSV-AP-ийн үндсэн property-үүдтэй тохирно: name, description,
-- competent authority, legal resource, target group, output, channel, cost,
-- processing time, status, life event.

CREATE TABLE IF NOT EXISTS registry_services (
    id               uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    code             TEXT UNIQUE NOT NULL,
    name             TEXT NOT NULL,
    name_en          TEXT NOT NULL DEFAULT '',
    description      TEXT NOT NULL DEFAULT '',
    authority        TEXT NOT NULL DEFAULT '',   -- эрх бүхий байгууллага
    authority_org_id uuid,                       -- organizations руу сул холбоос (FK-гүй: org нь RLS-тэй)
    legal_basis      TEXT NOT NULL DEFAULT '',   -- хууль зүйн үндэслэл (хууль/журмын заалт)
    target_group     TEXT NOT NULL DEFAULT '',   -- зорилтот бүлэг
    output           TEXT NOT NULL DEFAULT '',   -- гарах үр дүн
    channels         TEXT[] NOT NULL DEFAULT '{}', -- office/e-mongolia/mobile/phone/post
    fee              INT  NOT NULL DEFAULT 0,    -- MNT
    max_days         INT  NOT NULL DEFAULT 0,    -- хуулийн шийдвэрлэх дээд хугацаа
    steps_count      INT  NOT NULL DEFAULT 0,    -- процессын алхмын тоо
    annual_volume    INT  NOT NULL DEFAULT 0,    -- жилийн гүйлгээний тоо (SCM-ийн Q)
    proactivity      TEXT NOT NULL DEFAULT 'information', -- проактив байдлын шат
    status           TEXT NOT NULL DEFAULT 'draft',       -- draft | published | archived
    life_event_id    uuid REFERENCES registry_life_events(id) ON DELETE SET NULL,
    version          INT  NOT NULL DEFAULT 0,    -- сүүлд нийтлэгдсэн хувилбарын дугаар
    published_at     TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ,
    CONSTRAINT registry_services_status_chk      CHECK (status IN ('draft', 'published', 'archived')),
    CONSTRAINT registry_services_proactivity_chk CHECK (proactivity IN ('information', 'online', 'once_only', 'proactive')),
    CONSTRAINT registry_services_fee_chk         CHECK (fee >= 0),
    CONSTRAINT registry_services_days_chk        CHECK (max_days >= 0),
    CONSTRAINT registry_services_steps_chk       CHECK (steps_count >= 0),
    CONSTRAINT registry_services_volume_chk      CHECK (annual_volume >= 0)
);
CREATE INDEX IF NOT EXISTS idx_registry_services_status ON registry_services (status, name);
CREATE INDEX IF NOT EXISTS idx_registry_services_authority ON registry_services (authority);
CREATE INDEX IF NOT EXISTS idx_registry_services_life_event ON registry_services (life_event_id);

-- ── Паспорт ↔ нотолгоо (аль үйлчилгээ ямар бичиг баримт шаарддаг) ─────────
-- from_citizen = уг нотолгоог ИРГЭНЭЭС шаардаж байгаа эсэх (эсрэг тохиолдолд
-- байгууллага өөрөө системээс татаж авдаг).

CREATE TABLE IF NOT EXISTS registry_service_evidences (
    service_id   uuid NOT NULL REFERENCES registry_services(id)  ON DELETE CASCADE,
    evidence_id  uuid NOT NULL REFERENCES registry_evidences(id) ON DELETE CASCADE,
    required     BOOLEAN NOT NULL DEFAULT true,
    from_citizen BOOLEAN NOT NULL DEFAULT true,
    note         TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (service_id, evidence_id)
);
CREATE INDEX IF NOT EXISTS idx_registry_service_evidences_evidence ON registry_service_evidences (evidence_id);

-- ── Паспортын хувилбар + baseline delta (Казахстаны паспорт + ZGB хэмжилт) ─
-- Нийтлэлт бүр нэг мөр үүсгэнэ. is_baseline нь анхны (дахин инженерчлэлийн
-- өмнөх) төлөв — сайжралтыг ҮРГЭЛЖ энэ мөртэй харьцуулж хэмжинэ.

CREATE TABLE IF NOT EXISTS registry_service_versions (
    id              uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    service_id      uuid NOT NULL REFERENCES registry_services(id) ON DELETE CASCADE,
    version         INT  NOT NULL,
    snapshot        JSONB NOT NULL DEFAULT '{}',  -- нийтлэх үеийн паспортын бүтэн хуулбар
    change_note     TEXT NOT NULL DEFAULT '',
    is_baseline     BOOLEAN NOT NULL DEFAULT false,
    steps_count     INT NOT NULL DEFAULT 0,
    documents_count INT NOT NULL DEFAULT 0,
    max_days        INT NOT NULL DEFAULT 0,
    fee             INT NOT NULL DEFAULT 0,
    -- baseline-тай харьцуулсан ялгаа (сөрөг = сайжирсан).
    delta_steps     INT NOT NULL DEFAULT 0,
    delta_documents INT NOT NULL DEFAULT 0,
    delta_days      INT NOT NULL DEFAULT 0,
    delta_fee       INT NOT NULL DEFAULT 0,
    published_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_by    uuid,
    UNIQUE (service_id, version)
);
CREATE INDEX IF NOT EXISTS idx_registry_service_versions_service ON registry_service_versions (service_id, version DESC);

-- ── Once-only зөрчлийн харагдац ───────────────────────────────────────────
-- ХУР-д (эсвэл өөр төрийн санд) БАЙГАА мэдээллийг иргэнээс дахин шаардаж буй
-- бүх тохиолдол. Монголд хамгийн өндөр өгөөжтэй, хамгийн хэмжигдэхүйц дахин
-- инженерчлэлийн хэмжүүр (RING_SYSTEM_PLAN.md §R1).

CREATE OR REPLACE VIEW registry_once_only_violations AS
SELECT s.id                AS service_id,
       s.code              AS service_code,
       s.name              AS service_name,
       s.authority         AS authority,
       s.status            AS service_status,
       e.id                AS evidence_id,
       e.code              AS evidence_code,
       e.name              AS evidence_name,
       e.holder_agency     AS holder_agency,
       e.khur_service_code AS khur_service_code,
       s.annual_volume     AS annual_volume
FROM registry_service_evidences se
JOIN registry_services  s ON s.id = se.service_id
JOIN registry_evidences e ON e.id = se.evidence_id
WHERE se.from_citizen
  AND e.in_khur
  AND s.status <> 'archived';

-- ── Эрхийн каталог ────────────────────────────────────────────────────────
-- domain.PermRegistryView / PermRegistryManage-тэй тохирно. 'admin' нь бүх
-- каталогид авто-resolve хийдэг тул role_permissions мөр шаардлагагүй.

INSERT INTO permissions(key, label, category) VALUES
    ('registry.view',   'Үйлчилгээний регистр үзэх',    'administration'),
    ('registry.manage', 'Үйлчилгээний регистр удирдах', 'administration')
ON CONFLICT (key) DO NOTHING;

-- ── Seed (зөвхөн хоосон үед) ──────────────────────────────────────────────
-- Бодит нэвтрүүлэлтэд УБЕГ/ТЕГ/НДЕГ-ийн паспортоор солигдоно. Энд байгаа
-- жишээ нь платформыг эхний өдрөөс ажиллах чадвартай (once-only зөрчил
-- бодитоор илэрдэг) болгох зорилготой.

INSERT INTO registry_life_events(code, name, kind, description, lead_agency, sort_order)
SELECT * FROM (VALUES
    ('BIRTH',      'Хүүхэд төрөх',        'life',     'Төрөлт бүртгүүлэхээс тэтгэмж авах хүртэл.',      'УБЕГ', 10),
    ('MARRIAGE',   'Гэрлэх',              'life',     'Гэрлэлт бүртгүүлэх, хамтын өмч, хаяг өөрчлөх.',  'УБЕГ', 20),
    ('JOB_LOSS',   'Ажилгүй болох',       'life',     'Ажилгүйдлийн тэтгэмж, дахин сургалт, зуучлал.',  'ХНХЯ', 30),
    ('RETIREMENT', 'Тэтгэвэрт гарах',     'life',     'Тэтгэвэр тогтоох, эрүүл мэндийн даатгал.',       'НДЕГ', 40),
    ('BIZ_START',  'Бизнес эхлүүлэх',     'business', 'ААН бүртгэх, татвар, тусгай зөвшөөрөл.',         'УБЕГ', 50)
) AS v(code, name, kind, description, lead_agency, sort_order)
WHERE NOT EXISTS (SELECT 1 FROM registry_life_events);

INSERT INTO registry_evidences(code, name, description, holder_agency, source_system, in_khur, khur_service_code)
SELECT * FROM (VALUES
    ('EV_CIVIL_ID',   'Иргэний үнэмлэхний хуулбар',       'Иргэний бүртгэлийн үндсэн мэдээлэл.',       'УБЕГ',  'Иргэний бүртгэл',        true,  'WS100101_getCitizenIDCardInfo'),
    ('EV_RESIDENCE',  'Оршин суугаа газрын лавлагаа',     'Бүртгэлтэй хаягийн албан ёсны лавлагаа.',   'УБЕГ',  'Хаягийн бүртгэл',        true,  'WS100102_getCitizenAddressInfo'),
    ('EV_BIRTH_CERT', 'Төрсний гэрчилгээ',                'Төрөлтийн улсын бүртгэл.',                  'УБЕГ',  'Иргэний бүртгэл',        true,  'WS100103_getCitizenBirthInfo'),
    ('EV_TAX_CLEAR',  'Татварын өргүйн тодорхойлолт',     'Татварын өр төлбөрийн байдал.',             'ТЕГ',   'Татварын систем',        true,  'WS200101_getTaxpayerInfo'),
    ('EV_SOCIAL_INS', 'Нийгмийн даатгалын лавлагаа',      'Шимтгэл төлөлтийн дэлгэрэнгүй.',            'НДЕГ',  'НД-ын мэдээллийн сан',   true,  'WS300101_getInsuranceInfo'),
    ('EV_MARRIAGE',   'Гэрлэлтийн гэрчилгээ',             'Гэрлэлтийн улсын бүртгэл.',                 'УБЕГ',  'Иргэний бүртгэл',        true,  'WS100104_getCitizenMarriageInfo'),
    ('EV_DIPLOMA',    'Боловсролын диплом',               'Дээд боловсролын гэрчилгээ.',               'БШУЯ',  'Боловсролын бүртгэл',    false, ''),
    ('EV_MED_CERT',   'Эрүүл мэндийн үзлэгийн хуудас',    'Эмнэлгийн комиссын дүгнэлт.',               'ЭМЯ',   'Эрүүл мэндийн систем',   false, ''),
    ('EV_PHOTO',      'Цээж зураг',                       '3x4 хэмжээтэй цээж зураг.',                 '—',     '—',                      false, ''),
    ('EV_APPLICATION','Өргөдлийн маягт',                  'Иргэний бөглөсөн өргөдөл.',                 '—',     '—',                      false, '')
) AS v(code, name, description, holder_agency, source_system, in_khur, khur_service_code)
WHERE NOT EXISTS (SELECT 1 FROM registry_evidences);

INSERT INTO registry_services(code, name, name_en, description, authority, legal_basis, target_group, output,
                              channels, fee, max_days, steps_count, annual_volume, proactivity, status,
                              life_event_id, version, published_at)
SELECT v.code, v.name, v.name_en, v.description, v.authority, v.legal_basis, v.target_group, v.output,
       v.channels, v.fee, v.max_days, v.steps_count, v.annual_volume, v.proactivity, v.status,
       (SELECT id FROM registry_life_events le WHERE le.code = v.life_event_code),
       v.version, v.published_at
FROM (VALUES
    ('RS_CIVIL_ID', 'Иргэний үнэмлэх дахин олгох', 'Reissue of national ID card',
     'Гээгдсэн, гэмтсэн иргэний үнэмлэхийг дахин олгох.', 'УБЕГ',
     'Иргэний бүртгэлийн тухай хууль 12.3', 'Монгол Улсын иргэн', 'Иргэний үнэмлэх',
     ARRAY['office', 'e-mongolia'], 25000, 7, 6, 42000, 'online', 'published', 'BIRTH', 1, now()),

    ('RS_BIZ_REG', 'Аж ахуйн нэгж бүртгүүлэх', 'Business entity registration',
     'ХХК/ХК шинээр улсын бүртгэлд бүртгүүлэх.', 'УБЕГ',
     'Хуулийн этгээдийн улсын бүртгэлийн тухай хууль 9.1', 'Иргэн, хуулийн этгээд', 'Улсын бүртгэлийн гэрчилгээ',
     ARRAY['office', 'e-mongolia'], 44000, 10, 11, 18500, 'online', 'published', 'BIZ_START', 1, now()),

    ('RS_PENSION', 'Тэтгэвэр тогтоолгох', 'Pension entitlement',
     'Нас, хөдөлмөрийн чадвар алдалтын тэтгэвэр тогтоолгох.', 'НДЕГ',
     'Нийгмийн даатгалын сангаас олгох тэтгэврийн тухай хууль 4.1', 'Тэтгэврийн насны иргэн', 'Тэтгэврийн тогтоол',
     ARRAY['office'], 0, 20, 9, 31000, 'information', 'published', 'RETIREMENT', 1, now())
) AS v(code, name, name_en, description, authority, legal_basis, target_group, output,
       channels, fee, max_days, steps_count, annual_volume, proactivity, status, life_event_code, version, published_at)
WHERE NOT EXISTS (SELECT 1 FROM registry_services);

-- Шаардагдах нотолгоо. from_citizen=true + in_khur=true хослол нь once-only
-- зөрчил болж registry_once_only_violations-д гарч ирнэ (жишээ өгөгдөл дээр 6).
INSERT INTO registry_service_evidences(service_id, evidence_id, required, from_citizen, note)
SELECT s.id, e.id, v.required, v.from_citizen, ''
FROM (VALUES
    ('RS_CIVIL_ID', 'EV_APPLICATION', true,  true),
    ('RS_CIVIL_ID', 'EV_PHOTO',       true,  true),
    ('RS_CIVIL_ID', 'EV_RESIDENCE',   true,  true),   -- ⚠ ХУР-д байгаа
    ('RS_CIVIL_ID', 'EV_BIRTH_CERT',  false, true),   -- ⚠ ХУР-д байгаа

    ('RS_BIZ_REG',  'EV_APPLICATION', true,  true),
    ('RS_BIZ_REG',  'EV_CIVIL_ID',    true,  true),   -- ⚠ ХУР-д байгаа
    ('RS_BIZ_REG',  'EV_RESIDENCE',   true,  true),   -- ⚠ ХУР-д байгаа
    ('RS_BIZ_REG',  'EV_TAX_CLEAR',   true,  false),  -- ✓ системээс татдаг

    ('RS_PENSION',  'EV_APPLICATION', true,  true),
    ('RS_PENSION',  'EV_CIVIL_ID',    true,  true),   -- ⚠ ХУР-д байгаа
    ('RS_PENSION',  'EV_SOCIAL_INS',  true,  true),   -- ⚠ ХУР-д байгаа
    ('RS_PENSION',  'EV_MED_CERT',    false, true)
) AS v(service_code, evidence_code, required, from_citizen)
JOIN registry_services  s ON s.code = v.service_code
JOIN registry_evidences e ON e.code = v.evidence_code
WHERE NOT EXISTS (SELECT 1 FROM registry_service_evidences);

-- Анхны хувилбар = baseline (дахин инженерчлэлийн эхлэлийн цэг).
INSERT INTO registry_service_versions(service_id, version, snapshot, change_note, is_baseline,
                                      steps_count, documents_count, max_days, fee)
SELECT s.id, 1, '{}'::jsonb, 'Анхны бүртгэл (baseline)', true,
       s.steps_count,
       (SELECT count(*) FROM registry_service_evidences se WHERE se.service_id = s.id AND se.from_citizen),
       s.max_days, s.fee
FROM registry_services s
WHERE NOT EXISTS (SELECT 1 FROM registry_service_versions);
