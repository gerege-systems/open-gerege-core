-- Government Template Platform V3.0
-- 44_gov_service_workflow-ийг буцаана: workflow талбар, timeline, life event,
-- 'officer' RLS бодлого, шинэ эрхүүдийг устгаж каталогийн кодыг хуучин
-- дур мэдсэн түлхүүрүүд рүү нь буцаана.

-- Эрхүүд (role_permissions нь ON DELETE CASCADE тул permissions устгахад дагана).
DELETE FROM permissions WHERE key IN ('gov.review','gov.catalog');

-- 'officer' RLS бодлогууд.
DROP POLICY IF EXISTS gov_applications_officer  ON gov_applications;
DROP POLICY IF EXISTS gov_references_officer    ON gov_references;
DROP POLICY IF EXISTS gov_notifications_officer ON gov_notifications;

-- Timeline (өөрийн бодлогуудтайгаа хамт устана).
DROP TABLE IF EXISTS gov_application_events;

-- Life / business event.
DROP TABLE IF EXISTS gov_service_events;
DROP TABLE IF EXISTS gov_life_events;

-- Хүсэлтийн workflow талбарууд.
DROP INDEX IF EXISTS idx_gov_applications_queue;
DROP INDEX IF EXISTS idx_gov_applications_assignee;

-- Шинэ төлөвүүдтэй мөрүүдийг хуучин толинд буулгана — эс тэгвэл доорх
-- CHECK сэргээгдэхэд одоо байгаа өгөгдөл зөрчилд орно.
UPDATE gov_applications SET status = 'submitted' WHERE status IN ('registered','info_required');
UPDATE gov_applications SET status = 'cancelled' WHERE status = 'expired';

ALTER TABLE gov_applications DROP CONSTRAINT IF EXISTS gov_applications_status_check;
ALTER TABLE gov_applications DROP CONSTRAINT IF EXISTS gov_applications_result_check;

ALTER TABLE gov_applications
    DROP COLUMN IF EXISTS service_code,
    DROP COLUMN IF EXISTS assigned_to,
    DROP COLUMN IF EXISTS assigned_at,
    DROP COLUMN IF EXISTS decided_by,
    DROP COLUMN IF EXISTS decided_at,
    DROP COLUMN IF EXISTS decision_note,
    DROP COLUMN IF EXISTS result,
    DROP COLUMN IF EXISTS due_at,
    DROP COLUMN IF EXISTS sla_breached,
    DROP COLUMN IF EXISTS suspended_at,
    DROP COLUMN IF EXISTS payload,
    DROP COLUMN IF EXISTS output_ref_id,
    DROP COLUMN IF EXISTS tacit;

-- Каталогийн CPSV-AP талбарууд.
ALTER TABLE gov_services
    DROP CONSTRAINT IF EXISTS gov_services_fulfilment_check,
    DROP CONSTRAINT IF EXISTS gov_services_output_type_check,
    DROP CONSTRAINT IF EXISTS gov_services_assurance_check,
    DROP CONSTRAINT IF EXISTS gov_services_lifecycle_check;

-- 44-д шинээр нэмэгдсэн үйлчилгээ (хуучин seed-д байгаагүй) — устгана.
DELETE FROM gov_services WHERE code IN ('MN-0133-004','MN-0310-001');

-- Кодыг хуучин түлхүүрүүд рүү буцаана.
UPDATE gov_services SET code = 'CIVIL_ID'   WHERE code = 'MN-0133-001';
UPDATE gov_services SET code = 'RESIDENCE'  WHERE code = 'MN-0133-002';
UPDATE gov_services SET code = 'TAX_CLEAR'  WHERE code = 'MN-0112-001';
UPDATE gov_services SET code = 'SOCIAL_INS' WHERE code = 'MN-1090-001';
UPDATE gov_services SET code = 'DRIVER_LIC' WHERE code = 'MN-0451-001';
UPDATE gov_services SET code = 'MARRIAGE'   WHERE code = 'MN-0133-003';
UPDATE gov_services SET code = 'HEALTH_INS' WHERE code = 'MN-0721-001';
UPDATE gov_services SET code = 'BIZ_REG'    WHERE code = 'MN-0411-001';

ALTER TABLE gov_services
    DROP COLUMN IF EXISTS cofog_code,
    DROP COLUMN IF EXISTS cofog_label,
    DROP COLUMN IF EXISTS main_activity,
    DROP COLUMN IF EXISTS sdg_code,
    DROP COLUMN IF EXISTS processing_time,
    DROP COLUMN IF EXISTS output_type,
    DROP COLUMN IF EXISTS output_ref_type,
    DROP COLUMN IF EXISTS evidence,
    DROP COLUMN IF EXISTS legal_basis,
    DROP COLUMN IF EXISTS assurance_level,
    DROP COLUMN IF EXISTS lifecycle,
    DROP COLUMN IF EXISTS fulfilment,
    DROP COLUMN IF EXISTS has_discretion,
    DROP COLUMN IF EXISTS has_assessment,
    DROP COLUMN IF EXISTS sla_hours,
    DROP COLUMN IF EXISTS tacit_approval;
