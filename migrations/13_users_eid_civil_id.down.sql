-- Gerege Template Version 27.0
-- 13_users_eid_civil_id-ийг буцаана: civil_id дээрх partial unique index-ийг
-- устгана. Migration 12-ийн national_id index хэвээр үлдэнэ.
DROP INDEX IF EXISTS idx_users_civil_id_active;
