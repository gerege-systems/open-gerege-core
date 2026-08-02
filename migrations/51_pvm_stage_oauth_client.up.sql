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
