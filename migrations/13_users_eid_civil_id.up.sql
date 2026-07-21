-- Government Template Platform V3.0
-- Public RP-д eID IdP нь national_id (reg_no)-г илчлэхгүй, зөвхөн civil_id өгдөг
-- тул eID хэрэглэгчийн давтагдашгүй түлхүүр нь civil_id болно. UpsertFromEID-ийн
-- ON CONFLICT (lower(civil_id))-ийг дэмжих partial unique index нэмнэ. Migration
-- 12-ийн national_id index хэвээр үлдэнэ (эрх бүхий RP-ийн ховор тохиолдолд хор
-- хөнөөлгүй). civil_id нь жижиг үсгээр, зөвхөн утгатай (NULL биш) мөрүүд дээр
-- давтагдашгүй.
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_civil_id_active
    ON users(lower(civil_id))
    WHERE civil_id IS NOT NULL;
