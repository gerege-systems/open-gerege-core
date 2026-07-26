-- Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.
--
-- Корпусыг устгана (slug-тай бичлэгүүд). 11-р migration-ий жишээ бичлэгүүдийг
-- буцааж сэргээхгүй — тэдгээр нь энэ платформын хувьд буруу агуулгатай байсан.
DELETE FROM ai_knowledge WHERE slug IS NOT NULL;
