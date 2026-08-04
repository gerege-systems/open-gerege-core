-- 22_api_gateway-ийн буцаалт (gateway талын хүснэгтүүд).
-- applications / application_services нь applications модулийн down-д.
DROP TABLE IF EXISTS gateway_request_logs;
DROP TABLE IF EXISTS gateway_services;

DELETE FROM role_permissions WHERE permission_key = 'gateway.manage';
DELETE FROM permissions WHERE key = 'gateway.manage';
