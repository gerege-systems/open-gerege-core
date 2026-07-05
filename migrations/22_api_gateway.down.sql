-- Gerege Template Version 27.0
-- Reverse 14_api_gateway.up.sql. Drop in FK-dependency order; child tables
-- cascade, but explicit drops keep the intent clear.
DROP TABLE IF EXISTS gateway_request_logs;
DROP TABLE IF EXISTS gateway_policies;
DROP TABLE IF EXISTS gateway_api_keys;
DROP TABLE IF EXISTS gateway_routes;
DROP TABLE IF EXISTS gateway_consumers;
DROP TABLE IF EXISTS gateway_services;

DELETE FROM role_permissions WHERE permission_key = 'gateway.manage';
DELETE FROM permissions WHERE key = 'gateway.manage';
