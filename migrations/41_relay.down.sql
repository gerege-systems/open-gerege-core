-- Government Template Platform V3.0
-- Revert the inter-platform service-request relay + SLA monitor tables.

DROP TABLE IF EXISTS relay_events;
DROP TABLE IF EXISTS relay_assignments;
DROP TABLE IF EXISTS relay_requests;
DROP TABLE IF EXISTS relay_routes;
DROP TABLE IF EXISTS relay_platforms;

DELETE FROM permissions WHERE key IN ('relay.view', 'relay.manage');
