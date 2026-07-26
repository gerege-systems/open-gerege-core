-- Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.
--
-- ai_knowledge-ийг семантик (вектор) хайлтад бэлтгэнэ: pgvector extension,
-- embedding багана + HNSW индекс, мөн бичлэг бүрийн тогтвортой танигч (slug),
-- эх сурвалж, хэл, агуулгын hash.
--
-- Яагаад: ILIKE хайлт нь яг таарсан үгээр л оноо өгдөг тул «нэвтрэхэд юу
-- хэрэгтэй вэ?» гэх мэт өөр үгээр асуухад мэдлэгийн сангаас юу ч олдохгүй.
-- Embedding (утга санааны вектор) дээр cosine ойролцоолол хийвэл ижил утгатай
-- өөр үг хэллэгийг ч олно.
--
-- Extension нь суурь image-д эмхэтгэгдсэн байх ёстой — backend/deploy/db/
-- Dockerfile (postgres:16-alpine + pgvector) үүнийг хангана. Extension үүсгэх
-- нь superuser эрх шаарддаг тул migrate контейнер (POSTGRES_USER) хийнэ.
CREATE EXTENSION IF NOT EXISTS vector;

ALTER TABLE ai_knowledge
    -- slug: seed-ийг дахин ажиллуулахад давхардуулахгүй, тогтвортой түлхүүр.
    ADD COLUMN IF NOT EXISTS slug         TEXT,
    -- source: агуулга хаанаас гаралтай (docs/backend/frontend…) — хариултад
    -- эх сурвалж дурдах, засварлахад мөрдөх зориулалттай.
    ADD COLUMN IF NOT EXISTS source       TEXT,
    -- lang: корпусын хэл (одоогоор бүгд 'mn'; AI хариултаа UI-ийн хэл рүү
    -- орчуулж өгдөг тул нэг хэлээр хадгалж болно).
    ADD COLUMN IF NOT EXISTS lang         TEXT NOT NULL DEFAULT 'mn',
    -- embedding: text-embedding-004 → 768 хэмжээст вектор. NULL бол хараахан
    -- embed хийгдээгүй (backfill гүйцээнэ), тэр үед ILIKE fallback ажиллана.
    ADD COLUMN IF NOT EXISTS embedding    vector(768),
    -- content_hash: агуулга өөрчлөгдсөн эсэхийг мэдэж дахин embed хийхэд.
    ADD COLUMN IF NOT EXISTS content_hash TEXT,
    ADD COLUMN IF NOT EXISTS embedded_at  TIMESTAMPTZ;

-- slug нь давхардахгүй (seed нь ON CONFLICT (slug) DO UPDATE-ээр upsert хийнэ).
-- Partial индекс (WHERE slug IS NOT NULL) хийвэл ON CONFLICT түүнийг таньж
-- чадахгүй тул бүтэн unique индекс — Postgres дээр NULL-ууд хоорондоо
-- давхцахгүй тул хуучин slug-гүй мөрүүд саадгүй.
CREATE UNIQUE INDEX IF NOT EXISTS ai_knowledge_slug_key ON ai_knowledge (slug);

-- HNSW нь ойролцоо хөршийн индекс — cosine зайгаар (vector_cosine_ops).
-- Корпус жижиг үед ч зардалгүй; том болоход л жинхэнэ ашиг нь гарна.
CREATE INDEX IF NOT EXISTS ai_knowledge_embedding_hnsw
    ON ai_knowledge USING hnsw (embedding vector_cosine_ops);

-- Embedding-ийг backfill хийдэг нь api (app_user) тул тэр багануудад л
-- багана-түвшний UPDATE эрх өгнө. Агуулга (title/content/tags) нь migration/
-- seed-ээр удирдагдсан хэвээр — 17_least_privilege_config_grants-ийн REVOKE
-- хүчинтэй үлдэнэ.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_user') THEN
        GRANT UPDATE (embedding, content_hash, embedded_at) ON ai_knowledge TO app_user;
    END IF;
END $$;
