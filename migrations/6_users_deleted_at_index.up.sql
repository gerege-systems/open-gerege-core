-- Government Template Platform V3.0
-- soft-delete query-уудын гүйцэтгэлд зориулсан deleted_at индекс. GORM
-- AutoMigrate-ийг хассан (ORM-гүй) тул энэ индексийг ил тодорхойлно.
-- email/username нь partial-unique индексээр (WHERE deleted_at IS NULL)
-- хамгаалагдсан тул энэ нь голчлон List(IncludeDeleted) болон цэвэрлэгээний
-- query-уудад тус болно.
CREATE INDEX IF NOT EXISTS idx_users_deleted_at ON users (deleted_at);
