-- Government Template Platform V3.0
-- Өөрийн OAuth2/OIDC provider-ийн төлөв. Өмнө нь энэ бүхнийг Ory Hydra тусдаа
-- `hydra` database-д (superuser холболттой, RLS-ээс гадуур) хадгалдаг байсан.
-- Одоо үндсэн DB-д, RLS-ийн доор амьдарна.
--
-- Тэмдэглэл: OP session-д зориулсан тусдаа хүснэгт БАЙХГҮЙ — платформын одоо
-- байгаа httpOnly session (users/refresh token) нь OP session-ий үүргийг гүйцэтгэнэ.
-- Hydra өөрийн тусдаа cookie-тэй байсныг ингэж нэгтгэв.
--
-- Нууц утгууд (authorization code, access/refresh token) нь ЗӨВХӨН sha256 hash
-- хэлбэрээр хадгалагдана — DB задарсан ч токеныг сэргээх боломжгүй.

-- ── Client (relying party) бүртгэл ───────────────────────────────────────────
CREATE TABLE public.oauth_clients (
    client_id                  text PRIMARY KEY,
    client_name                text NOT NULL DEFAULT '',
    -- Public client (spa/native) бол хоосон. Формат: Ory-тай нийцтэй
    -- `$pbkdf2-sha256$i=...,l=...$salt$hash` эсвэл шинэ `$argon2id$...`.
    secret_hash                text NOT NULL DEFAULT '',
    token_endpoint_auth_method text NOT NULL DEFAULT 'client_secret_basic',
    app_type                   text NOT NULL DEFAULT 'm2m',
    grant_types                text[] NOT NULL DEFAULT '{}',
    response_types             text[] NOT NULL DEFAULT '{}',
    scopes                     text[] NOT NULL DEFAULT '{}',
    redirect_uris              text[] NOT NULL DEFAULT '{}',
    post_logout_redirect_uris  text[] NOT NULL DEFAULT '{}',
    tags                       text[] NOT NULL DEFAULT '{}',
    enabled                    boolean NOT NULL DEFAULT true,
    created_by                 text NOT NULL DEFAULT '',
    created_at                 timestamp with time zone NOT NULL DEFAULT now(),
    updated_at                 timestamp with time zone,
    CONSTRAINT oauth_clients_app_type_chk
        CHECK (app_type IN ('web', 'spa', 'native', 'm2m')),
    CONSTRAINT oauth_clients_auth_method_chk
        CHECK (token_endpoint_auth_method IN ('client_secret_basic', 'client_secret_post', 'none'))
);

-- ── id_token гарын үсгийн түлхүүр ────────────────────────────────────────────
-- private_key_enc нь INTEGRATION_ENC_KEY-ээр AES-256-GCM шифрлэгдсэн PKCS#8
-- (pkg/crypto → base64(nonce||ciphertext) тул text).
CREATE TABLE public.oauth_signing_keys (
    kid             text PRIMARY KEY,
    alg             text NOT NULL DEFAULT 'RS256',
    private_key_enc text NOT NULL,
    public_jwk      jsonb NOT NULL,
    active          boolean NOT NULL DEFAULT true,
    created_at      timestamp with time zone NOT NULL DEFAULT now(),
    retired_at      timestamp with time zone
);

-- Идэвхтэй (гарын үсэг зурдаг) түлхүүр аль ч үед ЗӨВХӨН нэг байна; retire
-- хийсэн түлхүүрүүд JWKS-д хэвээр үлдэж, хуучин id_token шалгагдсаар байна.
CREATE UNIQUE INDEX oauth_signing_keys_one_active
    ON public.oauth_signing_keys (active) WHERE active;

-- ── Authorization code ───────────────────────────────────────────────────────
CREATE TABLE public.oauth_auth_codes (
    code_hash             bytea PRIMARY KEY,
    client_id             text NOT NULL REFERENCES public.oauth_clients(client_id) ON DELETE CASCADE,
    subject               uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    scopes                text[] NOT NULL DEFAULT '{}',
    redirect_uri          text NOT NULL,
    nonce                 text NOT NULL DEFAULT '',
    code_challenge        text NOT NULL DEFAULT '',
    code_challenge_method text NOT NULL DEFAULT '',
    auth_time             timestamp with time zone NOT NULL DEFAULT now(),
    expires_at            timestamp with time zone NOT NULL,
    -- Дахин ашиглалтыг илрүүлэхийн тулд code-ыг УСТГАХГҮЙ, зөвхөн тэмдэглэнэ
    -- (RFC 6749 §4.1.2 — хоёр дахь удаа ирвэл гэрлийг нь бүлгээр нь цуцална).
    used_at               timestamp with time zone,
    created_at            timestamp with time zone NOT NULL DEFAULT now()
);

CREATE INDEX idx_oauth_auth_codes_expires ON public.oauth_auth_codes (expires_at);

-- ── Access token (opaque) ────────────────────────────────────────────────────
-- subject нь client_credentials grant-д NULL (хэрэглэгчгүй).
CREATE TABLE public.oauth_access_tokens (
    token_hash    bytea PRIMARY KEY,
    client_id     text NOT NULL REFERENCES public.oauth_clients(client_id) ON DELETE CASCADE,
    subject       uuid REFERENCES public.users(id) ON DELETE CASCADE,
    scopes        text[] NOT NULL DEFAULT '{}',
    refresh_family uuid,
    expires_at    timestamp with time zone NOT NULL,
    revoked_at    timestamp with time zone,
    created_at    timestamp with time zone NOT NULL DEFAULT now()
);

CREATE INDEX idx_oauth_access_tokens_expires ON public.oauth_access_tokens (expires_at);
CREATE INDEX idx_oauth_access_tokens_subject ON public.oauth_access_tokens (subject, client_id);
CREATE INDEX idx_oauth_access_tokens_family  ON public.oauth_access_tokens (refresh_family);

-- ── Refresh token (эргэлттэй) ────────────────────────────────────────────────
-- family_id нь эргэлтийн бүх үеийг нэгтгэнэ: хэрэглэгдсэн token дахин ирвэл
-- (хулгайлагдсаны шинж) БҮХ бүлгийг цуцална.
CREATE TABLE public.oauth_refresh_tokens (
    token_hash   bytea PRIMARY KEY,
    family_id    uuid NOT NULL,
    rotated_from bytea,
    client_id    text NOT NULL REFERENCES public.oauth_clients(client_id) ON DELETE CASCADE,
    subject      uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    scopes       text[] NOT NULL DEFAULT '{}',
    nonce        text NOT NULL DEFAULT '',
    auth_time    timestamp with time zone NOT NULL DEFAULT now(),
    expires_at   timestamp with time zone NOT NULL,
    consumed_at  timestamp with time zone,
    revoked_at   timestamp with time zone,
    created_at   timestamp with time zone NOT NULL DEFAULT now()
);

CREATE INDEX idx_oauth_refresh_tokens_family  ON public.oauth_refresh_tokens (family_id);
CREATE INDEX idx_oauth_refresh_tokens_expires ON public.oauth_refresh_tokens (expires_at);
CREATE INDEX idx_oauth_refresh_tokens_subject ON public.oauth_refresh_tokens (subject, client_id);

-- ── Login / consent / logout challenge ───────────────────────────────────────
-- Authorize хүсэлтийн параметрүүдийг хадгалж, нэвтрэх/зөвшөөрөх UI-аас буцаж
-- ирэхэд сэргээнэ. Challenge нь нэг удаагийн, богино хугацаатай.
CREATE TABLE public.oauth_challenges (
    challenge               text PRIMARY KEY,
    kind                    text NOT NULL,
    client_id               text REFERENCES public.oauth_clients(client_id) ON DELETE CASCADE,
    subject                 uuid REFERENCES public.users(id) ON DELETE CASCADE,
    requested_scopes        text[] NOT NULL DEFAULT '{}',
    granted_scopes          text[] NOT NULL DEFAULT '{}',
    redirect_uri            text NOT NULL DEFAULT '',
    state                   text NOT NULL DEFAULT '',
    nonce                   text NOT NULL DEFAULT '',
    response_type           text NOT NULL DEFAULT '',
    code_challenge          text NOT NULL DEFAULT '',
    code_challenge_method   text NOT NULL DEFAULT '',
    prompt                  text NOT NULL DEFAULT '',
    post_logout_redirect_uri text NOT NULL DEFAULT '',
    skip                    boolean NOT NULL DEFAULT false,
    decided_at              timestamp with time zone,
    expires_at              timestamp with time zone NOT NULL,
    created_at              timestamp with time zone NOT NULL DEFAULT now(),
    CONSTRAINT oauth_challenges_kind_chk CHECK (kind IN ('login', 'consent', 'logout'))
);

CREATE INDEX idx_oauth_challenges_expires ON public.oauth_challenges (expires_at);

-- ── Санагдсан зөвшөөрөл (consent) ────────────────────────────────────────────
CREATE TABLE public.oauth_consents (
    subject    uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    client_id  text NOT NULL REFERENCES public.oauth_clients(client_id) ON DELETE CASCADE,
    scopes     text[] NOT NULL DEFAULT '{}',
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    updated_at timestamp with time zone,
    PRIMARY KEY (subject, client_id)
);

CREATE INDEX idx_oauth_consents_expires ON public.oauth_consents (expires_at);

-- ── RLS ──────────────────────────────────────────────────────────────────────
-- ХУВААРЬ:
--   * oauth_clients, oauth_signing_keys — системийн тохиргоо, хэрэглэгчид
--     хамааралгүй. Одоо байгаа `applications` / `gateway_services`-ийн адил
--     RLS-ГҮЙ; зөвшөөрлийг route давхарга (RequirePermission gateway.manage)
--     шийднэ. ЯАГААД: `gateway.manage` эрхтэй ч админ БИШ хэрэглэгч RoleUser
--     авдаг (middleware_auth.go:188-190) тул admin/service-only бодлого түүнийг
--     чимээгүй хаах байсан. Түлхүүрийн нууцлалыг AES-GCM шифрлэлт хамгаална.
--   * Доорх хэрэглэгч-тус-бүрийн хүснэгтүүд RLS-тэй. Протоколын endpoint-ууд
--     (authorize/token/userinfo) нэвтрэхээс ӨМНӨ ажилладаг тул RoleService-ээр
--     хандана; иргэн өөрийн зөвшөөрлөө харна (oauth_consents_self).

ALTER TABLE public.oauth_auth_codes ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.oauth_auth_codes FORCE ROW LEVEL SECURITY;
CREATE POLICY oauth_auth_codes_service ON public.oauth_auth_codes
    USING ((current_setting('app.user_role', true) = 'service'))
    WITH CHECK ((current_setting('app.user_role', true) = 'service'));

ALTER TABLE public.oauth_access_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.oauth_access_tokens FORCE ROW LEVEL SECURITY;
CREATE POLICY oauth_access_tokens_service ON public.oauth_access_tokens
    USING ((current_setting('app.user_role', true) = 'service'))
    WITH CHECK ((current_setting('app.user_role', true) = 'service'));
CREATE POLICY oauth_access_tokens_admin ON public.oauth_access_tokens
    USING ((current_setting('app.user_role', true) = 'admin'))
    WITH CHECK ((current_setting('app.user_role', true) = 'admin'));

ALTER TABLE public.oauth_refresh_tokens ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.oauth_refresh_tokens FORCE ROW LEVEL SECURITY;
CREATE POLICY oauth_refresh_tokens_service ON public.oauth_refresh_tokens
    USING ((current_setting('app.user_role', true) = 'service'))
    WITH CHECK ((current_setting('app.user_role', true) = 'service'));
CREATE POLICY oauth_refresh_tokens_admin ON public.oauth_refresh_tokens
    USING ((current_setting('app.user_role', true) = 'admin'))
    WITH CHECK ((current_setting('app.user_role', true) = 'admin'));

ALTER TABLE public.oauth_challenges ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.oauth_challenges FORCE ROW LEVEL SECURITY;
CREATE POLICY oauth_challenges_service ON public.oauth_challenges
    USING ((current_setting('app.user_role', true) = 'service'))
    WITH CHECK ((current_setting('app.user_role', true) = 'service'));

ALTER TABLE public.oauth_consents ENABLE ROW LEVEL SECURITY;
ALTER TABLE public.oauth_consents FORCE ROW LEVEL SECURITY;
CREATE POLICY oauth_consents_service ON public.oauth_consents
    USING ((current_setting('app.user_role', true) = 'service'))
    WITH CHECK ((current_setting('app.user_role', true) = 'service'));
CREATE POLICY oauth_consents_admin ON public.oauth_consents
    USING ((current_setting('app.user_role', true) = 'admin'))
    WITH CHECK ((current_setting('app.user_role', true) = 'admin'));
CREATE POLICY oauth_consents_self ON public.oauth_consents
    USING (((current_setting('app.user_role', true) = 'user')
        AND (subject = (NULLIF(current_setting('app.user_id', true), ''))::uuid)))
    WITH CHECK (((current_setting('app.user_role', true) = 'user')
        AND (subject = (NULLIF(current_setting('app.user_id', true), ''))::uuid)));
