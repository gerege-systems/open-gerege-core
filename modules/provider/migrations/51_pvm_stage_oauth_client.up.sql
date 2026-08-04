-- pvm.stagegerege.mn relying party OAuth2 client registration

INSERT INTO public.oauth_clients (
    client_id,
    client_name,
    token_endpoint_auth_method,
    app_type,
    grant_types,
    response_types,
    scopes,
    redirect_uris,
    enabled,
    created_by
) VALUES (
    'pvm-stagegerege-mn',
    'Gerege Production Environment',
    'none',
    'web',
    ARRAY['authorization_code', 'refresh_token']::text[],
    ARRAY['code']::text[],
    ARRAY['openid', 'profile', 'email']::text[],
    ARRAY['https://pvm.stagegerege.mn/sso/callback']::text[],
    true,
    'seed-rp'
) ON CONFLICT (client_id) DO UPDATE SET
    redirect_uris = EXCLUDED.redirect_uris,
    client_name = EXCLUDED.client_name;

-- `public.applications` нь Hydra-гийн үеийн ХУУЧИН overlay хүснэгт. Хэрэглэгч
-- платформууд түүнийг өөрсдийн migration-аар УСТГАСАН байж болно (жишээ нь
-- gerege-platform-mn-ий `1003_drop_legacy_applications` — client-ууд одоо
-- зөвхөн `oauth_clients`-д амьдардаг). Тэдгээр дээр болзолгүй INSERT нь
--
--     ERROR: relation "public.applications" does not exist (SQLSTATE 42P01)
--
-- өгч migration-ыг унагаадаг тул deploy БҮХЭЛДЭЭ гацна (2026-08-03-нд
-- gerege-platform-mn дээр хэмжигдсэн). Иймд хүснэгт БАЙГАА тохиолдолд Л бичнэ.
DO $$
BEGIN
    IF to_regclass('public.applications') IS NOT NULL THEN
        INSERT INTO public.applications (
            client_id,
            name,
            app_type,
            tags,
            redirect_uris,
            enabled,
            created_by
        ) VALUES (
            'pvm-stagegerege-mn',
            'pvm.stagegerege.mn',
            'web',
            ARRAY['rp', 'stage']::text[],
            ARRAY['https://pvm.stagegerege.mn/sso/callback']::text[],
            true,
            'seed-rp'
        ) ON CONFLICT (client_id) DO UPDATE SET
            redirect_uris = EXCLUDED.redirect_uris,
            name = EXCLUDED.name;
    END IF;
END $$;

-- Ensure template-gerege-mn also accepts pvm.stagegerege.mn callback as alias
UPDATE public.oauth_clients
SET redirect_uris = ARRAY(SELECT DISTINCT unnest(redirect_uris || ARRAY['https://pvm.stagegerege.mn/sso/callback']::text[]))
WHERE client_id = 'template-gerege-mn';
