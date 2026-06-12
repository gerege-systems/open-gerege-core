-- Gerege Template Version 27.0
-- Reverses 15_audit_log.up.sql — drops both event-store tables (policies are
-- dropped implicitly with the tables).

DROP TABLE IF EXISTS security_events;
DROP TABLE IF EXISTS audit_log;
