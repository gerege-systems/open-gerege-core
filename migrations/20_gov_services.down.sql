-- Government Template Platform V3.0
-- Reverse 20_gov_services.up.sql (RLS бодлого + хүснэгтүүд).

DROP POLICY IF EXISTS gov_appointments_self    ON gov_appointments;
DROP POLICY IF EXISTS gov_appointments_admin   ON gov_appointments;
DROP POLICY IF EXISTS gov_appointments_service ON gov_appointments;
DROP POLICY IF EXISTS gov_payments_self        ON gov_payments;
DROP POLICY IF EXISTS gov_payments_admin       ON gov_payments;
DROP POLICY IF EXISTS gov_payments_service     ON gov_payments;
DROP POLICY IF EXISTS gov_notifications_self   ON gov_notifications;
DROP POLICY IF EXISTS gov_notifications_admin  ON gov_notifications;
DROP POLICY IF EXISTS gov_notifications_service ON gov_notifications;
DROP POLICY IF EXISTS gov_references_self      ON gov_references;
DROP POLICY IF EXISTS gov_references_admin     ON gov_references;
DROP POLICY IF EXISTS gov_references_service   ON gov_references;
DROP POLICY IF EXISTS gov_applications_self    ON gov_applications;
DROP POLICY IF EXISTS gov_applications_admin   ON gov_applications;
DROP POLICY IF EXISTS gov_applications_service ON gov_applications;

ALTER TABLE gov_appointments  NO FORCE ROW LEVEL SECURITY;
ALTER TABLE gov_appointments  DISABLE  ROW LEVEL SECURITY;
ALTER TABLE gov_payments      NO FORCE ROW LEVEL SECURITY;
ALTER TABLE gov_payments      DISABLE  ROW LEVEL SECURITY;
ALTER TABLE gov_notifications NO FORCE ROW LEVEL SECURITY;
ALTER TABLE gov_notifications DISABLE  ROW LEVEL SECURITY;
ALTER TABLE gov_references    NO FORCE ROW LEVEL SECURITY;
ALTER TABLE gov_references    DISABLE  ROW LEVEL SECURITY;
ALTER TABLE gov_applications  NO FORCE ROW LEVEL SECURITY;
ALTER TABLE gov_applications  DISABLE  ROW LEVEL SECURITY;

DROP TABLE IF EXISTS gov_appointments;
DROP TABLE IF EXISTS gov_payments;
DROP TABLE IF EXISTS gov_notifications;
DROP TABLE IF EXISTS gov_references;
DROP TABLE IF EXISTS gov_applications;
DROP TABLE IF EXISTS gov_services;
