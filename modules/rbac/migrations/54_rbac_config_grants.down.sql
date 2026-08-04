-- Grant-уудыг сэргээнэ (зөвхөн хөгжүүлэлт/тестэд).
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'app_user') THEN
        RETURN;
    END IF;
    GRANT INSERT, UPDATE, DELETE ON permissions TO app_user;
    GRANT UPDATE ON role_permissions TO app_user;
END $$;
