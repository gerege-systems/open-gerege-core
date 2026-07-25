-- Gerege Template Platform V3.0
-- Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_user') THEN
        REVOKE UPDATE (embedding, content_hash, embedded_at) ON ai_knowledge FROM app_user;
    END IF;
END $$;

DROP INDEX IF EXISTS ai_knowledge_embedding_hnsw;
DROP INDEX IF EXISTS ai_knowledge_slug_key;

ALTER TABLE ai_knowledge
    DROP COLUMN IF EXISTS embedded_at,
    DROP COLUMN IF EXISTS content_hash,
    DROP COLUMN IF EXISTS embedding,
    DROP COLUMN IF EXISTS lang,
    DROP COLUMN IF EXISTS source,
    DROP COLUMN IF EXISTS slug;

-- Extension-ыг үлдээнэ: өөр объект хамааралтай байж болзошгүй, мөн дахин
-- эмхэтгэх шаардлагагүй. Бүрэн устгах бол гараар: DROP EXTENSION vector;
