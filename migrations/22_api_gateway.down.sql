-- Government Template Platform V3.0
-- 22_api_gateway-ийн буцаалт. FK-хамаарлын дарааллаар устгана (child эхэлж).
DROP TABLE IF EXISTS application_services;
DROP TABLE IF EXISTS applications;
DROP TABLE IF EXISTS gateway_request_logs;
DROP TABLE IF EXISTS gateway_services;

DELETE FROM role_permissions WHERE permission_key = 'gateway.manage';
DELETE FROM permissions WHERE key = 'gateway.manage';
