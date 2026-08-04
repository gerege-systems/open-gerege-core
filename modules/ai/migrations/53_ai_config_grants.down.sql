-- Grant-уудыг сэргээнэ (зөвхөн хөгжүүлэлт/тестэд — production rollback нь
-- expand–contract бодлогоор binary-аар явна).
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_user') THEN
        RETURN;
    END IF;
    GRANT INSERT, DELETE ON ai_prompts TO app_user;
    GRANT INSERT, UPDATE, DELETE ON ai_knowledge TO app_user;
END $$;
