-- Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.
--
-- Gateway схемийн нэгтгэл (forward-only, idempotent reconcile).
--
-- 22_api_gateway.up.sql-ийг ГАЗАР ДЭЭР нь дахин бичсэн (анхны routes/consumers/
-- api_keys/policies схемээс → нэгдсэн applications схем рүү) бөгөөд завсрын
-- 29-34 migration-уудыг устгасан. Migration runner нь хэрэгжсэн migration-ыг
-- ЗӨВХӨН файлын нэрээр тэмдэглэдэг тул хуучин 22-г бүртгэсэн DB дахин бичсэн
-- 22-г ХЭЗЭЭ Ч дахин гүйцэтгэхгүй, устгагдсан 29-34 ч гүйхгүй. Үр дүнд ийм DB
-- нь gateway_services.scope багана, applications / application_services
-- хүснэгтгүй үлдэж, gateway/applications admin API бүхэлдээ 500 өгдөг.
--
-- Энэ migration нь ДУРЫН өмнөх төлвийг (хуучин 22, эсвэл эцсийн схем) эцсийн
-- схем рүү нэгтгэнэ. Аль хэдийн нэгтгэсэн DB дээр бүрэн no-op (IF [NOT] EXISTS).

-- 1) gateway_services.scope — аппад олгох OAuth scope нэр.
ALTER TABLE gateway_services ADD COLUMN IF NOT EXISTS scope TEXT NOT NULL DEFAULT '';

-- 2)/3) applications ба application_services-ийн нэгтгэл нь applications
-- модуль руу нүүсэн (modules/applications/migrations/36_applications_reconcile).

-- 4) Хуучин 22-гийн хэрэглэгддэггүй gateway plumbing хүснэгтүүдийг устгана
-- (нэгдсэн gateway эдгээрийг ашиглахаа больсон). Эцсийн схемтэй DB дээр аль
-- хэдийн байхгүй тул IF EXISTS → no-op. CASCADE нь gateway_request_logs-ийн
-- route_id/consumer_id FK-г цэвэрлэнэ (багана өөрөө үлдэнэ, хоосон/нөлөөгүй).
DROP TABLE IF EXISTS gateway_policies;
DROP TABLE IF EXISTS gateway_api_keys;
DROP TABLE IF EXISTS gateway_routes CASCADE;
DROP TABLE IF EXISTS gateway_consumers CASCADE;

-- Хуучин request-log-ийн хэрэглэгддэггүй багануудыг цэвэрлэнэ (эцсийн схемд байхгүй).
ALTER TABLE gateway_request_logs DROP COLUMN IF EXISTS route_id;
ALTER TABLE gateway_request_logs DROP COLUMN IF EXISTS consumer_id;

-- 5) API Gateway admin surface-ийн permission (аль хэдийн байвал no-op).
INSERT INTO permissions(key, label, category) VALUES
    ('gateway.manage', 'API Gateway удирдах', 'administration')
ON CONFLICT (key) DO NOTHING;
