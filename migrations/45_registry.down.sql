-- Government Template Platform V3.0
-- Ring System · R1 — Үйлчилгээний нэгдсэн регистрийг буцаана.
-- Хамаарлын эсрэг дарааллаар: view → хамаарсан хүснэгтүүд → мастер хүснэгтүүд.

DROP VIEW  IF EXISTS registry_once_only_violations;

DROP TABLE IF EXISTS registry_service_versions;
DROP TABLE IF EXISTS registry_service_evidences;
DROP TABLE IF EXISTS registry_services;
DROP TABLE IF EXISTS registry_evidences;
DROP TABLE IF EXISTS registry_life_events;

DELETE FROM permissions WHERE key IN ('registry.view', 'registry.manage');
