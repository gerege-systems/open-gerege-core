-- Government Template Platform V3.0
-- Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.
--
-- 36_gateway_reconcile нь forward-only нэгтгэл (аль ч өмнөх төлвийг эцсийн
-- схем рүү авчирна). Үүнийг буцаах нь утгагүй: applications / application_services
-- / gateway_services.scope нь 22_api_gateway-ийн ЭЗЭМШИЛД байдаг тул тэдгээрийг
-- 22-гийн down устгана. Устгасан хуучин plumbing хүснэгтүүдийг (routes/consumers/
-- api_keys/policies) утга төгөлдөр сэргээх боломжгүй. Тиймээс энэ down нь
-- зориудаар no-op — зөвхөн schema_migrations-аас 36-гийн мөрийг runner хасна.
SELECT 1;
