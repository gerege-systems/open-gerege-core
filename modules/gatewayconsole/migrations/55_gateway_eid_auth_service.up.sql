-- Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.
--
-- eID НЭВТРЭЛТИЙН proxy (/v1/eid-auth/*) — gateway каталогийн бүртгэл.
--
-- Энэ мөр байснаар admin консолоос service-ийг асаах/унтраах, апп тус бүрд
-- "svc:eid-auth" эрхийг олгох боломжтой болно. Каталогт байхгүй үед route нь
-- fail-open (идэвхтэй) ажилладаг тул энэ migration нь ХАРАГДАХ БАЙДЛЫН төлөө —
-- эрхийн шалгалт (svc:eid-auth) нь каталогоос үл хамааран хүчинтэй хэвээр.
INSERT INTO gateway_services (name, protocol, host, port, path, tags, scope)
SELECT 'eid-auth', 'https', 'sso.gerege.mn', 443, '/rp/eid-auth',
       ARRAY['eid', 'auth']::text[], 'svc:eid-auth'
WHERE NOT EXISTS (SELECT 1 FROM gateway_services WHERE name = 'eid-auth');
