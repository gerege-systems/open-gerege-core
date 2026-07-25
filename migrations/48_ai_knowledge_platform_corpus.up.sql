-- Gerege Template Platform V3.0
-- Gerege Systems Development Team болон Claude AI хамтран бүтээв, 2026.
--
-- Платформын мэдлэгийн корпус — код болон баримтжуулалтаас (backend/docs,
-- docs/, docs-site/, frontend) нэгтгэсэн бодит агуулга. AI туслах эдгээрийг
-- search_knowledge tool-оор семантик хайж, хариултаа ЭНД байгаа өгөгдөлд
-- тулгуурлана (таамаглахгүй).
--
-- Дүрэм:
--   * slug нь тогтвортой түлхүүр — дахин ажиллуулахад upsert хийнэ.
--   * Агуулга өөрчлөгдвөл embedding-ийг NULL болгож дахин embed хийлгэнэ.
--   * Бүх бичлэг монголоор — AI хариултаа хэрэглэгчийн хэл рүү орчуулна.

-- 11-р migration-ий жишээ бичлэгүүд (нууц үг сэргээх, OTP бүртгэл) нь энэ
-- платформд БУРУУ: нууц үг / OTP-ээр нэвтрэх урсгал огт байхгүй. Тэдгээрийг
-- устгаж, доорх бодит корпусаар солино.
DELETE FROM ai_knowledge WHERE slug IS NULL AND id IN (1, 2, 3);

INSERT INTO ai_knowledge (slug, title, content, tags, source, lang) VALUES

-- ── Платформын ерөнхий ─────────────────────────────────────────────────────
('platform-overview', 'Gerege Template Platform гэж юу вэ',
 'Gerege Template Platform V3.0 бол цахим үйлчилгээг дээр нь босгох, үйлдвэрлэлд бэлэн суурь платформ. Төрийн байгууллага ч, банк, даатгал, финтек, эрүүл мэнд, боловсролын хувийн хэвшлийн бүтээгдэхүүн ч нэг суурин дээр ижил түвшний баталгаажуулалт, аюулгүй байдалтайгаар босдог. Бүрэлдэхүүн: Clean Architecture-тэй Go backend, Next.js BFF frontend, Gemini AI pipeline. Хэрэглэгч танилт (eID), аюулгүй байдал, AI, үйлчилгээний тулгуурыг дахин бүтээхгүйгээр шууд ашиглана. MIT лицензтэй нээлттэй эх.',
 '{платформ,тойм,танилцуулга}', 'docs/README', 'mn'),

('platform-stack', 'Технологийн стек',
 'Backend: Go, chi (net/http) router, pgx (pgxpool) драйвер дээр гар бичмэл SQL — ORM ашигладаггүй, PostgreSQL өгөгдлийн сан, Redis + Ristretto хоёр давхаргат кэш. Frontend: Next.js 15 App Router (React 19, server components), TypeScript, TanStack Query. AI: Gemini REST (SDK-гүй). Танилт: eID Mongolia Relying Party, Google OAuth, өөрийн OIDC provider. Ажиглалт: OpenTelemetry trace, Prometheus metrics, Zap структурт лог. Тест: unit (mockery mock) + testcontainers интеграци.',
 '{стек,технологи,go,nextjs,postgres}', 'docs/README', 'mn'),

('clean-architecture', 'Clean Architecture давхаргууд',
 'Хамаарал зөвхөн дотогшоо чиглэнэ: handler → usecase → repository → domain. HTTP давхарга usecase-ийн интерфэйсээс, usecase нь repositories/interface-ээс (пакет нэр _interface) хамаарна — хэзээ ч postgres адаптераас биш. Domain давхарга нь стандарт сан болон bcrypt-ээс өөр юу ч импортлохгүй. Ингэснээр business/domain код web framework-ийг мэдэхгүй тул хүргэлтийн давхаргыг солиход бизнес логик хөндөгдөхгүй. Ганц зориудын үл хамаарах зүйл: internal/datasources/rls нь зөвхөн context ашигладаг навч пакет учир гурван давхаргад хуваалцагдана.',
 '{архитектур,давхарга,clean architecture}', 'backend/docs/ARCHITECTURE', 'mn'),

('monorepo-layout', 'Репозиторын бүтэц',
 'Монорепо: backend/ (Go API, migrations, docs), frontend/ (Next.js BFF), docs-site/ (MkDocs Material сайт), ios/TemplateApp (SwiftUI жишиг клиент), docs/ (deploy, аюулгүй байдал, хувь нэмэр), deploy/ (nginx, deploy.sh). Backend дотор: cmd/api/server/server.go нь бүх холболтын цэг (гар DI), internal/business (domain + 19 usecase), internal/datasources (pgx, кэш, RLS, репозитор), internal/http (handler, middleware, route, DTO), pkg/ (eid, google, gemini, hydra, xyp, gspace, jwt г.м.).',
 '{бүтэц,монорепо,directory}', 'backend/docs/ARCHITECTURE', 'mn'),

('reference-deployment', 'Жишиг deployment-ууд',
 'template.gerege.mn нь энэ платформын эталон deployment — eID нэвтрэлтийг production-д харуулна, өөрөө Gerege SSO-ийн relying party. sso.gerege.mn нь Gerege SSO — OIDC provider ба eID Relying Party (eID креденшлийг зөвхөн тэр эзэмшинэ). Холбогдсон аппууд sso.gerege.mn-ээр нэвтэрч, eID үйлчилгээг прокси-оор дуудна.',
 '{deployment,sso,template.gerege.mn}', 'docs-site/index', 'mn'),

-- ── Нэвтрэлт ──────────────────────────────────────────────────────────────
('auth-no-password', 'Нууц үгээр нэвтрэх БАЙХГҮЙ',
 'Энэ платформ дээр нууц үг, имэйл/OTP-ээр нэвтрэх, бүртгүүлэх, нууц үг сэргээх урсгал ОГТ байхгүй. Цорын ганц интерактив нэвтрэлт бол eID. Backend-ийн кодод auth_login.go, auth_register.go, auth_send_otp.go зэрэг хуучин файл үлдсэн ч ямар ч route-д холбогдоогүй (route_auth.go зөвхөн eID / Google / refresh / logout бүртгэдэг). Иймд «нууц үгээ мартсан» гэх мэт зөвлөгөө өгөх нь буруу — хэрэглэгчийг eID-ээр нэвтрэхийг заана.',
 '{нэвтрэлт,нууц үг,password,байхгүй}', 'backend/docs/SECURITY', 'mn'),

('auth-eid-overview', 'eID-ээр нэвтрэх',
 'Нэвтрэлт нь eID Mongolia-гийн Relying Party хэлбэрээр явагдана. Гурван арга: (1) QR код — дэлгэц дээрх кодыг гар утасны eID апп-аар уншуулна, (2) регистрийн дугаараар — иргэний бүртгэлтэй төхөөрөмж рүү push мэдэгдэл очно, (3) App2App — нэг утсан дээр eID апп руу шууд шилжинэ. Аль ч тохиолдолд баталгаажуулалтын код гарч ирэх ба утсан дээрх кодтой тааруулж зөвшөөрнө. Амжилттай болмогц хэрэглэгч civil_id-гаар upsert хийгдэж, JWT токен хос үүснэ.',
 '{eid,нэвтрэлт,qr,push}', 'backend/docs/API_CONTRACT', 'mn'),

('auth-eid-endpoints', 'eID нэвтрэлтийн endpoint-ууд',
 'POST /api/v1/auth/eid/start — QR / mobile deep-link сесс эхлүүлнэ. POST /api/v1/auth/eid/start-id — регистрийн дугаараар push илгээнэ. POST /api/v1/auth/eid/poll — frontend ~2.5 секунд тутам long-poll хийж (IdP тал 25 секунд хүртэл барина) сесс COMPLETE болтол хүлээнэ; COMPLETE үед access + refresh токен буцна. Poll-д тусдаа зөөлөн лимит (~60/мин) тавьсан — урт хүлээлт өөрөө 429 болох ёсгүй. Бусад /auth/* нь ~5/мин, body 4 KiB-ээр хязгаарлагдана.',
 '{eid,endpoint,poll,api}', 'backend/docs/API_CONTRACT', 'mn'),

('auth-google-link', 'Google данс холбох',
 'Google-ээр эхлээд шууд нэвтрэх боломжгүй: эхний удаад eID-ээр өөрийгөө баталгаажуулж, Google дансаа жинхэнэ хүнтэй холбоно. Дараагаас нь Google-ээр нэг товшилтоор нэвтэрнэ. POST /api/v1/auth/google нь authorization code-ыг сервер талд солиж холбоно; DELETE /api/v1/auth/google/link нь салгана. GOOGLE_CLIENT_ID тохируулаагүй бол уг товч идэвхгүй харагдана.',
 '{google,oauth,холболт}', 'backend/docs/ARCHITECTURE', 'mn'),

('auth-session-jwt', 'Session — JWT access ба refresh',
 'Нэвтэрсэн хэрэглэгч access JWT (богино насалттай) болон refresh JWT авна. POST /auth/refresh нь токен хосыг эргэлдүүлнэ (rotation) — хуучин refresh хүчингүй болно; kind claim нь refresh токеныг access болгож ашиглахаас хамгаална. Нууц үг/креденшл солигдсоноос өмнөх токенууд TokensRevokedBefore-оор татгалзана. POST /auth/logout нь refresh-ийг устгаж, access-ийн jti-г үлдсэн хугацаагаар Redis deny-list-д тавьдаг тул тэр даруй ажиллахаа болино.',
 '{session,jwt,refresh,logout}', 'backend/docs/ARCHITECTURE', 'mn'),

('auth-sso-consumer', 'Gerege SSO-оор нэвтрэх (OIDC consumer)',
 'Апп нь sso.gerege.mn-ийн relying party болж чадна: /api/auth/sso/start → backend Redis-д state үүсгэж authorize URL руу шилжүүлнэ → sso.gerege.mn дээр eID-ээр баталгаажина → /sso/callback руу code+state буцаж ирнэ → backend токен солиод sso_sub-аар хэрэглэгчийг upsert хийж, өөрийн session-ээ гаргана. Тохиргоо: SSO_ISSUER, SSO_CLIENT_ID, SSO_CLIENT_SECRET, SSO_REDIRECT_URI, SSO_SCOPE. Мобайл нативд PKCE-тэй public client (SSO_NATIVE_CLIENT_ID).',
 '{sso,oidc,нэвтрэлт,consumer}', 'docs-site/sso-integration', 'mn'),

-- ── eID PKI профайл ───────────────────────────────────────────────────────
('eid-profile', 'eID PKI профайл',
 'Нэвтэрсэн иргэн өөрийн eID мэдээллээ харна: /api/v1/users/me/eid/summary (ерөнхий), /certificates (гэрчилгээ), /devices (холбоотой төхөөрөмж), /activity (нэвтрэлт/гарын үсгийн түүх). Frontend дээр /me/eid/id, /me/eid/certificates, /me/eid/devices, /me/eid/logs, /me/eid/security хуудсууд. Эдгээр өгөгдөл eidmongolia.mn-ээс ирдэг тул RP-д PKI_READ зөвшөөрөл олгогдсон байх шаардлагатай; олгогдоогүй бол хуудас хүлээгдэж буй тухай мэдэгдэнэ.',
 '{eid,pki,профайл,гэрчилгээ,төхөөрөмж}', 'backend/docs/API_CONTRACT', 'mn'),

('eid-organizations', 'Байгууллага холбох',
 'Иргэн өөрийн төлөөлдөг байгууллагаа регистрийн дугаараар холбоно: POST /api/v1/users/me/eid/organizations. Байгууллагыг улсын бүртгэлээс (XYP / Gerege Verify) шалгаж, eID дээрх төлөөллийн эрхийг баталгаажуулна — зөвхөн захирал, үүсгэн байгуулагч, хувьцаа эзэмшигч зэрэг эрх бүхий хүн холбож чадна. DELETE .../organizations/{regNo} нь салгана. Frontend: /me/organizations болон /me/organizations/eid/{regNo}.',
 '{байгууллага,eid,xyp,регистр}', 'backend/docs/API_CONTRACT', 'mn'),

('eid-signers', 'Байгууллагын гарын үсэг зурагчид',
 'Байгууллагын ADMIN нь бусад хүнийг гарын үсэг зурах эрхтэй болгож нэмнэ: POST /api/v1/users/me/eid/organizations/{regNo}/signers (регистрийн дугаар, албан тушаал, хамтарсан/дангаар эрх). Нэмэгдсэн хүнд eID баталгаажуулалтын хүсэлт очих ба тэр хүн PIN-ээр зөвшөөрсний дараа идэвхтэй болно (түүнээс өмнө «хүлээгдэж буй» төлөвтэй). Хүсэлтийг дахин илгээх, эсвэл эрхийг хасах боломжтой.',
 '{гарын үсэг,signer,байгууллага,eid}', 'backend/docs/API_CONTRACT', 'mn'),

('eid-sign-pades', 'Баримтад цахим гарын үсэг зурах (PAdES)',
 'PDF баримтыг eID Mongolia /v3-ээр сервер талаас PAdES стандартаар гарын үсэг зурна: POST /api/v1/sign/init сесс эхлүүлж, GET /api/v1/sign/{id} төлөвийг асууж, GET /api/v1/sign/{id}/download гарын үсэгтэй PDF-ийг татна. Утсан дээрх eID апп-аас PIN2-оор зөвшөөрнө. Хувь хүн өөрийн нэрээр, эсвэл төлөөлдөг байгууллагынхаа нэрээр зурж болно — сүүлийн тохиолдолд eidmongolia төлөөллийн эрхийг шалгана. Frontend: /me/eid/sign.',
 '{гарын үсэг,pades,pdf,sign}', 'backend/docs/API_CONTRACT', 'mn'),

('sign-relay', 'Гарын үсгийн зуучлал (sign relay)',
 'Гуравдагч RP-ууд өөрсдөө eID гэрчилгээ эзэмшихгүйгээр платформын eID креденшлээр ДАМЖУУЛАН баримтад гарын үсэг зуруулж чадна. Энэ нь /rp/sign/* дээрх reverse proxy (internal/provider/signrelay) — дуудагч тал хуваалцсан токеныг Authorization: Bearer-ээр өгөхөд relay нь жинхэнэ eID RP secret-ээр солиод eID Mongolia руу дамжуулна. SIGN_RELAY_TOKEN болон EID_RP_SECRET хоёулаа тохируулагдсан үед л идэвхжинэ.',
 '{sign relay,rp,гарын үсэг}', 'backend/docs/ARCHITECTURE', 'mn'),

-- ── OIDC provider ─────────────────────────────────────────────────────────
('oidc-provider-overview', 'Платформ өөрөө OIDC provider болох',
 'Платформ нь өөрийн Go OAuth2/OIDC provider-оор бусад аппын нэвтрэлтийг хангаж чадна. Login / consent / logout урсгалыг өөрөө жолоодож, иргэнийг eID-ээр баталгаажуулаад subject-ыг олгоно. Идэвхжих нөхцөл: OAUTH_ISSUER (эсвэл Hydra тохиргоо) байх ба SSO_STATE_KEY ≥ 32 байт. Тохируулаагүй бол энэ гадаргуу огт бүртгэгдэхгүй. Frontend талд /oauth/login, /oauth/consent, /oauth/logout, /oauth/error хуудсууд.',
 '{oidc,provider,sso,consent}', 'backend/docs/ARCHITECTURE', 'mn'),

('oidc-rp-register', 'Апп (RP) бүртгэх',
 'Админ → Applications хэсэгт OAuth2 client бүртгэнэ. app_type нь урсгалыг тодорхойлно: web (authorization_code, нууцтай), spa болон native (public client, PKCE, secret-гүй), m2m (client_credentials). client_secret нь зөвхөн үүсгэх / rotate хийх үед НЭГ УДАА харагдана — дараа нь дахин үзэх боломжгүй. Redirect URI яг таарах ёстой; логоут ажиллахын тулд post-logout redirect URI-г мөн бүртгэнэ.',
 '{applications,oauth2,client,rp}', 'docs-site/sso-integration', 'mn'),

('eid-service-proxy', 'eID Service Proxy',
 'Бүртгэлтэй апп нь өөрөө eID креденшл эзэмшихгүйгээр SSO-ийн eID үйлчилгээг прокси-оор дуудна. Хоёр сервис: eid-proxy (хувь хүн — summary, certificates, devices, activity, зам /rp/eid/*) ба eid-org-proxy (байгууллага — organizations, signers, зам /rp/eid-org/*). Бүгд зөвхөн уншина. Дуудалт бүрд SSO нь токеныг introspect хийж, тухайн client-д svc:eid-proxy / svc:eid-org-proxy scope олгогдсон эсэхийг шалгана: токенгүй бол 401, эрх олгогдоогүй бол 403, сервис унтраалттай бол 503.',
 '{eid proxy,scope,rp,503}', 'docs-site/eid-services', 'mn'),

-- ── API Gateway ───────────────────────────────────────────────────────────
('api-gateway', 'API Gateway',
 'Админ удирддаг сервисийн каталог ба телеметр. Админ → Gateway хэсэгт сервисүүдийг жагсаах, үүсгэх, засах, идэвхжүүлэх/унтраах боломжтой; сервис үүсгэхэд svc:<нэр> scope автоматаар үүсдэг. Overview нь сүүлийн 24 цагийн ачаалал, алдаа, латентыг харуулах ба Logs нь бодит хүсэлтүүдийг (арга, зам, статус, латент, client_ip) жагсаана — эдгээрийг middleware бодитоор бичдэг.',
 '{gateway,сервис,телеметр}', 'docs-site/api-gateway', 'mn'),

('gateway-service-grants', 'Сервисийн эрх олголт',
 'Апп-д сервис олгох гэдэг нь тухайн OAuth2 client-ийн зөвшөөрөгдсөн scope-д svc:<нэр>-ийг нэмэх явдал (Админ → Applications → SERVICES). Хасахад scope устана. Энэ нь тэр даруй үйлчилнэ — прокси хүсэлт бүрд одоогийн эрхийг шалгадаг. Нэвтрэлт (SSO login) нь суурь үйлчилгээ тул бүх бүртгэлтэй аппд автоматаар нээлттэй, тусад нь олгох шаардлагагүй.',
 '{scope,эрх,applications,gateway}', 'docs-site/api-gateway', 'mn'),

-- ── Төрийн үйлчилгээ, байгууллага ─────────────────────────────────────────
('gov-portal', 'Төрийн үйлчилгээний портал',
 'Иргэн рүү харсан «Төрийн үйлчилгээ» гадаргуу: үйлчилгээний каталог, хүсэлт (application), лавлагаа, мэдэгдэл, төлбөр, цаг захиалга. Бүх өгөгдөл хэрэглэгч тус бүрийнх — токеноос гарсан userID-гаар шүүгдэж, Postgres RLS-ээр давхар хамгаалагдана. Endpoint-ууд /api/v1/gov/* дор; frontend дээр /me/services, /me/applications, /me/references, /me/notifications, /me/payments, /me/appointments.',
 '{gov,төрийн үйлчилгээ,хүсэлт,лавлагаа}', 'backend/docs/API_CONTRACT', 'mn'),

('gov-actions', 'Хүсэлт, төлбөр, цаг захиалгын үйлдлүүд',
 'Хүсэлт илгээх: POST /gov/applications; цуцлах: POST /gov/applications/{id}/cancel. Лавлагаа захиалах: POST /gov/references. Мэдэгдлийг уншсан болгох: POST /gov/notifications/{id}/read эсвэл /gov/notifications/read-all. Хүлээгдэж буй төлбөр төлөх: POST /gov/payments/{id}/pay. Цаг захиалах: POST /gov/appointments, цуцлах: /gov/appointments/{id}/cancel. Эдгээр бичих үйлдлүүд ~30 хүсэлт/минут лимиттэй.',
 '{gov,төлбөр,цаг захиалга,мэдэгдэл}', 'backend/docs/API_CONTRACT', 'mn'),

('org-management', 'Байгууллага ба гишүүнчлэл',
 'Байгууллага үүсгэх, регистрийн дугаараар хайх (улсын бүртгэлээс лавлана), гишүүд болон тэдний эрхийг удирдах боломжтой: /api/v1/org/*. Гишүүний үүрэг: эзэмшигч (owner), админ, гишүүн. Гишүүн нэмэх/хасах эрхийг usecase давхарга шалгана — эрхгүй бол «танд энэ байгууллагад гишүүн нэмэх, хасах эрх алга» гэсэн алдаа гарна. Бүх мөр RLS-ээр хамгаалагдсан.',
 '{байгууллага,гишүүн,эрх,org}', 'backend/docs/API_CONTRACT', 'mn'),

-- ── Интеграци, хадгалалт ──────────────────────────────────────────────────
('integrations-oauth', 'Гуравдагч интеграцууд',
 'Хэрэглэгч өөрийн Google Drive, Google Meet, Dropbox дансаа профайлдаа холбож, баримт болон уулзалтаа нэг дороос удирдана: /api/v1/integrations/*. Токенууд хэрэглэгч тус бүрээр AES-256-GCM-ээр шифрлэгдэн хадгалагдаж (INTEGRATION_ENC_KEY түлхүүр), RLS-ээр тусгаарлагдана. OAuth креденшл тохируулаагүй интеграц нь «удахгүй» төлөвтэй идэвхгүй харагдана — тухайн үйлчилгээ рүү огт хандахгүй.',
 '{интеграци,google drive,dropbox,шифрлэлт}', 'backend/docs/ARCHITECTURE', 'mn'),

('gerege-space', 'Gerege Space хадгалалт',
 'Апп-ын өөрийн SFTP хадгалалт — хэрэглэгч бүрд квоттой (өгөгдмөл 2 MB). /api/v1/gspace/ нь ашиглалт, квот, файлын жагсаалтыг өгнө; upload, download, delete үйлдлүүдтэй. GSPACE_* тохиргоо (host, user, password, base path, quota) хоосон бол энэ боломж идэвхгүй — endpoint нь 500 буцаана.',
 '{gspace,хадгалалт,sftp,квот}', 'backend/docs/API_CONTRACT', 'mn'),

('assets-signature-stamp', 'Гарын үсэг ба тамганы зураг',
 'Хэрэглэгч гарын үсгийнхээ зургийг байршуулж, баримт гарын үсэглэхэд ашиглана (зураг нь хэрэглэгчийн Google Drive-д хадгалагдана). Байгууллагын тамганы зургийг зөвхөн байгууллагын ADMIN байршуулна. Зураг нь PNG/JPG байх ба 1MB-аас бага. Endpoint: /api/v1/me/signature, /api/v1/me/orgstamp/{regNo}. Мөн латин нэрээ (автомат галиглалт буруу байвал) /api/v1/me/latin-name-ээр гараар засна.',
 '{гарын үсэг,тамга,зураг,латин нэр}', 'backend/docs/API_CONTRACT', 'mn'),

-- ── AI туслах ─────────────────────────────────────────────────────────────
('ai-assistant-overview', 'AI туслах юу хийдэг вэ',
 'AI туслах нь Gemini дээр суурилсан бөгөөд платформын талаарх асуултад мэдлэгийн санд тулгуурлаж хариулна. Модел ямар хэрэгсэл (tool) дуудахаа шийднэ, backend түүнийг гүйцэтгэнэ — модел өөрөө хэзээ ч код ажиллуулахгүй. Хэрэгслүүд хүсэлтийн контекстээр ажилладаг тул RLS болон timeout хамаарна. Frontend: /me/ai хуудас; текст болон дуут мессеж дэмжинэ, хариултыг чанга уншуулж болно.',
 '{ai,туслах,gemini,tool}', 'backend/docs/AI_PIPELINE', 'mn'),

('ai-language-rule', 'AI ямар хэлээр хариулах вэ',
 'Хариултын хэлийг интерфейсийн хэл тодорхойлно: frontend нь mn / en / zh / ru кодыг хүсэлт бүрд дамжуулна. Туслах ЗӨВХӨН тэр хэлээр хариулна — хэрэглэгчийн бичсэн хэл, ярианы өмнөх түүх, мэдлэгийн сангийн эх бичвэрийн хэл, tool-ийн үр дүн аль нь ч үүнийг өөрчлөхгүй; өөр хэл дээрх эх сурвалжийг орчуулж өгнө. Зөвхөн хэрэглэгч шууд хүсвэл хэлээ солино. Хэл тодорхойгүй бол монгол хэл өгөгдмөл.',
 '{ai,хэл,орчуулга,mn en zh ru}', 'backend/docs/AI_PIPELINE', 'mn'),

('ai-prompt-layers', 'AI-ийн prompt давхаргууд',
 'System prompt гурван давхаргаас угсарна: (1) суурь хамгаалалт — кодод хатуу бичигдсэн, хэзээ ч тохируулагдахгүй (хариултын хэл, хамрах хүрээний сахилт, prompt-injection эсэргүүцэл, зааврыг задлахгүй); (2) хамрах хүрээ (scope) — ai_prompts хүснэгтээс, админ ажиллаж байх үед өөрчилнө; (3) нэмэлт заавар (instructions) — сонголттой, өнгө аяс зэрэг. Админ → Тохиргоо хэсгээс (settings.manage эрхтэй) 2, 3-р давхаргыг л засна.',
 '{ai,prompt,guardrail,scope}', 'backend/docs/AI_PIPELINE', 'mn'),

('ai-knowledge-base', 'Мэдлэгийн сан ба семантик хайлт',
 'Платформын мэдлэг ai_knowledge хүснэгтэд бүлэг бүлгээр хадгалагдана. Бичлэг бүр Gemini-ийн text-embedding-004 моделиор 768 хэмжээст вектор болж хувирч, pgvector дээр cosine ойролцоо хайлтаар (HNSW индекс) хайгддаг. Ингэснээр хэрэглэгч өөр үг хэллэгээр асуусан ч утга санааны хувьд ойр бүлгүүд олдоно. Embedding хараахан үүсээгүй эсвэл Gemini түлхүүр байхгүй үед систем ILIKE (түлхүүр үгийн) хайлт руу уналт хийнэ.',
 '{ai,мэдлэгийн сан,vector,embedding,pgvector}', 'backend/docs/AI_PIPELINE', 'mn'),

('ai-answer-variety', 'AI яагаад ижил асуултад өөр өөр хариулдаг вэ',
 'Туслах нь ижил асуултад бүрэн адилхан үг хэллэгээр хариулахгүй: хүсэлт бүрд найруулгын өөр хэв маяг (эхлэл, бүтэц, жишээ, дараалал) сонгож, generation тохиргоо нь ч давтагдлыг бууруулна. Гэхдээ энэ нь зөвхөн НАЙРУУЛГА — баримт, тоо, алхмууд, эх сурвалж өөрчлөгдөхгүй. Тодорхой алхам жагсаах шаардлагатай асуултад агуулга нь тогтвортой хэвээр байна.',
 '{ai,хариулт,найруулга,давтагдахгүй}', 'backend/docs/AI_PIPELINE', 'mn'),

('ai-voice', 'Дуу хоолойн боломжууд',
 'Дуут мессеж: микрофоноор бичээд илгээхэд аудио шууд модельд очно (тусдаа STT алхам шаардахгүй, модел олон төрлийн оролт ойлгоно). Яриа→текст: POST /ai/stt. Текст→яриа: POST /ai/tts — WAV буцаадаг тул хөтөч дээр шууд тоглоно. Шууд орчуулга: /me/translate хуудас микрофоны ~7 секундын хэсгүүдийг /ai/translate руу дараалан илгээж бодит цагт орчуулна; чимээгүй хэсэг хоосон ирвэл алгасна.',
 '{ai,дуу хоолой,stt,tts,орчуулга}', 'backend/docs/AI_PIPELINE', 'mn'),

('ai-limits', 'AI-ийн хязгаарлалт ба алдаа',
 '/ai/* нь нэг IP-аас минутад ~20 хүсэлт зөвшөөрнө (шууд орчуулга минутад ~8 хэсэг илгээдэг тул багтана). Мессеж 4000 тэмдэгт, түүх 20 ээлж, аудио ~700 KB base64-ээр хязгаарлагдана. Gemini түр зуур ажиллахгүй бол хүсэлт 5xx болж унахгүй — хэрэглэгчид уучлалт хүссэн нөөц мессеж (degraded) очно. GEMINI_API_KEY огт тохируулаагүй бол л жинхэнэ 500 алдаа гарна.',
 '{ai,лимит,degraded,алдаа}', 'backend/docs/AI_PIPELINE', 'mn'),

-- ── Аюулгүй байдал ────────────────────────────────────────────────────────
('security-rls', 'Мөр бүрийн хандалтын хяналт (RLS)',
 'Хэрэглэгчийн өгөгдөл бүхий хүснэгт бүрд Postgres Row-Level Security ENABLE + FORCE хэлбэрээр асаалттай: users, organizations, organization_memberships, gov_* болон user_integrations. Хүсэлтийн эзэн SET LOCAL app.user_id / app.user_role GUC-ээр гүйлгээ бүрд дамжина (withRLS). Эзэнгүй хүсэлт GUC хоосон үлдээх тул ямар ч бодлого таарахгүй — бүх мөр нуугдаж, бичих үйлдэл татгалзана (fail-closed). Энэ нь репозиторын WHERE нөхцөлийн ДЭЭР нэмэгдсэн давхар хамгаалалт.',
 '{аюулгүй байдал,rls,postgres,тусгаарлалт}', 'backend/docs/SECURITY', 'mn'),

('security-rls-boot-guard', 'RLS-ийн boot guard',
 'Postgres дээр superuser болон BYPASSRLS эрхтэй роль RLS-ийг чимээгүй тойрдог. Тиймээс апп асахдаа өөрийн DB ролийг pg_roles-оос шалгана: production-д superuser / BYPASSRLS бол boot зогсоно (fail closed), development-д зөвхөн анхааруулга бичнэ. Иймд api нь заавал app_user гэх мэт хамгийн бага эрхтэй роль-оор холбогдох ёстой; migrate л superuser хэвээр (CREATE EXTENSION, RLS DDL хийхэд).',
 '{аюулгүй байдал,rls,boot guard,superuser}', 'backend/docs/SECURITY', 'mn'),

('security-headers-csrf', 'Вэб хамгаалалт: толгой, CORS, CSRF',
 'Хариу бүрд аюулгүй байдлын толгойнууд: CSP, HSTS (production), nosniff, X-Frame-Options: DENY, Referrer-Policy, Permissions-Policy, COOP/COEP/CORP. CORS нь ALLOWED_ORIGINS-ийн яг таарах жагсаалтаар ажиллах ба хэзээ ч * + credentials хослуулахгүй. Frontend нь BFF загвартай тул токен httpOnly cookie-д үлдэж browser-ийн JS-д хэзээ ч гарахгүй; төлөв өөрчлөх дуудалт бүр x-dgov-csrf толгой + Origin шалгалт гэсэн давхар CSRF хамгаалалттай.',
 '{аюулгүй байдал,csp,cors,csrf,bff}', 'backend/docs/SECURITY', 'mn'),

('security-audit-log', 'Аудит бүртгэл',
 'Аудит лог нь hash-chain холбоост, зөвхөн нэмэх (append-only): chain_hash = SHA-256(өмнөх hash ‖ бичлэгийн каноник JSON). Бичигчид pg_advisory_xact_lock-оор дараалдаг тул гинж эмх цэгцтэй. GET /api/v1/audit нь бичлэгүүдийг, GET /api/v1/audit/verify нь гинжний бүрэн бүтэн байдлыг шалгана — зассан, устгасан ул мөр шууд илэрнэ. Зөвхөн админ уншина. Супер админы бүх үйлдэл (админ үүсгэх, эрх олгох/хасах) энд бичигдэнэ.',
 '{аудит,hash chain,лог,админ}', 'backend/docs/SECURITY', 'mn'),

('security-rbac-roles', 'Эрхийн систем (RBAC) ба үүргүүд',
 'Дөрвөн зэрэглэлтэй үүрэг (1 нь хамгийн өндөр): супер админ = 1, админ = 2, менежер = 3, хэрэглэгч = 4. Админ бүх зөвшөөрлийг автоматаар эзэмшинэ; бусад үүргийн зөвшөөрлийг Админ → Эрх (RBAC) хэсгээс динамикаар тохируулна. Route-ууд RequirePermission / RequireAdmin / RequireSuperAdmin middleware-ээр хамгаалагдана. GET /api/v1/rbac/me нь хэрэглэгчийн бодит зөвшөөрлүүдийг өгдөг — цэсийг үүгээр шүүнэ.',
 '{rbac,эрх,үүрэг,role}', 'backend/docs/ARCHITECTURE', 'mn'),

('security-superadmin', 'Супер админ',
 'Супер админ бол админ хэрэглэгчдийг удирдах цорын ганц үүрэг: /api/v1/superadmin/* дор админ үүсгэх, байгаа хэрэглэгчид админ эрх олгох, эрх хасах. Энгийн админ энэ гадаргууд орж чадахгүй (RequireSuperAdmin). Супер админыг API-аар хэзээ ч үүсгэдэггүй — SUPERADMIN_EMAIL орчны хувьсагчаар (аль хэдийн eID-ээр нэвтэрсэн хэрэглэгчийг дараагийн ачаалалтад дэвшүүлнэ) эсвэл өгөгдлийн санд role_id=1 болгож эхлүүлнэ.',
 '{супер админ,эрх,бүртгэл}', 'backend/README', 'mn'),

('security-secrets', 'Нууц түлхүүрүүдийн журам',
 'backend/internal/config/.env*, root .env болон backend.env нь gitignore-д — хэзээ ч commit хийхгүй. Шинэ хувьсагч нэмбэл README-д баримтжуулна (утгыг нь биш). INTEGRATION_ENC_KEY нь заавал шаардлагатай бөгөөд НЭГ УДАА тавьсны дараа өөрчлөхгүй — сольвол өмнө шифрлэсэн бүх утга (интеграцийн токен, супер админы MFA) сэргээгдэхгүй болно. JWT_SECRET солих нь бүх session-ийг хүчингүй болгоно.',
 '{нууц үг,env,түлхүүр,шифрлэлт}', 'docs-site/configuration', 'mn'),

('security-observability-gate', 'Ажиглалтын endpoint-уудын хаалт',
 'Production-д /metrics болон /swagger/doc.json нь Authorization: Bearer <OBSERVABILITY_TOKEN> шаардана; тохирохгүй эсвэл токен тохируулаагүй бол 401 биш 404 буцаана — endpoint байгаа эсэх нь ч танигдахгүй. Development-д нээлттэй. /health (амьд эсэх) ба /ready (DB + Redis) нь балансын хэрэгсэлд зориулж үргэлж нээлттэй.',
 '{metrics,swagger,404,ажиглалт}', 'backend/docs/ARCHITECTURE', 'mn'),

-- ── Frontend ──────────────────────────────────────────────────────────────
('frontend-bff', 'Frontend BFF загвар',
 'Хөтөч зөвхөн ижил origin дээрх Next.js route (/api/*) руу хандана; тэдгээр нь сервер талаас Go API руу прокси хийнэ. Токен httpOnly cookie-д (dgov_access, dgov_refresh) хадгалагдаж клиент JS-д хэзээ ч гарахгүй. 401 хүлээж авбал refresh токеноор нэг удаа автоматаар шинэчилж дахин оролдоно; refresh нь rotation хийдэг тул cookie бичих боломжгүй контекстод (RSC render) огт хийгдэхгүй. Backend-ийн хариунаас зөвхөн нууц БУС талбарууд клиент рүү дамжина.',
 '{frontend,bff,cookie,токен}', 'frontend/README', 'mn'),

('frontend-i18n', 'Интерфейсийн хэлүүд',
 'Интерфейс дөрвөн хэлтэй: монгол (өгөгдмөл), англи, хятад, орос. Хэрэглэгч баруун дээд булангийн цэснээс сонгоно; сонголт localStorage-д хадгалагдаж, дараагийн айлчлалд ч хэвээр байна. Нүүр хуудсанд хэлний товч mn → en → zh → ru гэж эргэлдэнэ. Бүх мөр frontend/src/lib/i18n.ts дахь түлхүүрээр гарах ба түлхүүр бүр дөрвөн хэлэнд байх ёстой (тест шалгана).',
 '{хэл,i18n,интерфейс,орчуулга}', 'frontend/README', 'mn'),

('frontend-appearance', 'Харагдац ба загвар',
 'Хэрэглэгч өөрийн харагдацаа тохируулна: гэрэл/харанхуй/системийн загвар, өнгөний өргөлт (cobalt, teal, violet, emerald, amber), фонт (Inter, serif, системийн), нягтрал (comfortable, compact). Эдгээр нь localStorage-д хадгалагдаж, хуудас зурагдахаас өмнө тусгагдана (анивчихгүй). Админ нь нэвтрээгүй зочдод харагдах нийтийн хуудсуудын өгөгдмөл харагдацыг тусад нь тохируулна, мөн нүүр хуудасны бүрэн загварыг (өнгө, текст, цэс) theme болгон үүсгэж идэвхжүүлж болно.',
 '{харагдац,theme,өнгө,фонт}', 'frontend/README', 'mn'),

('frontend-routes', 'Үндсэн хуудсууд',
 'Нэвтэрсэн хэрэглэгч: /me/dashboard (самбар), /me/profile (профайл), /me/settings (тохиргоо), /me/ai (AI туслах), /me/translate (шууд орчуулга), /me/eid/* (үнэмлэх, гэрчилгээ, төхөөрөмж, түүх, аюулгүй байдал, гарын үсэг), /me/organizations, /me/integrations болон төрийн үйлчилгээний хуудсууд. Админ: /admin/dashboard, /admin/users, /admin/roles, /admin/superadmin, /admin/audit, /admin/security, /admin/settings, /admin/core, /admin/gateway/*. Менежер: /manager/dashboard, /manager/users.',
 '{хуудас,route,цэс,навигаци}', 'frontend/README', 'mn'),

-- ── Ажиллагаа, deploy ─────────────────────────────────────────────────────
('deploy-compose', 'Deploy — Docker Compose',
 'Бүтэн стек нэг VPS дээр Docker Compose + nginx-ээр ажиллана: db (Postgres + pgvector), redis, migrate (нэг удаагийн), api, web. docker compose up -d --build нь бүгдийг өргөнө; migrate нь ачаалал бүрд ажиллаж, аль хэдийн хийгдсэн migration-ыг алгасна (идемпотент). TLS-ийг nginx дээр certbot-оор терминацлана. Хоёр env файл: .env (compose-ийн утга) ба backend.env (API-ийн тохиргоо) — хоёулаа gitignore-д.',
 '{deploy,docker,compose,nginx}', 'docs/DEPLOYMENT', 'mn'),

('deploy-ci-cd', 'CI/CD ба автомат deploy',
 'GitHub Actions дээр CI нь gofmt, go vet, race тест, swagger дрейф шалгалт, frontend lint/build/тест, gitleaks нууц скан хийнэ. Эдгээр ногоон болсны дараа тусдаа deploy ажил SSH-ээр VPS руу орж, CI амжилттай болсон яг тэр commit руу reset хийж, deploy.sh (rebuild → up -d → эрүүл болтол хүлээх → prune) ажиллуулна. Улаан build хэзээ ч production-д гарахгүй. Гараар зэрэг deploy хийвэл контейнерийн нэр давхцаж алдаа өгч болзошгүй.',
 '{ci,deploy,github actions,pipeline}', 'docs/DEPLOYMENT', 'mn'),

('deploy-db-roles', 'Хоёр DB роль яагаад хэрэгтэй вэ',
 'RLS-ийг superuser чимээгүй тойрдог тул стек хоёр роль ашиглана: migrate (болон hydra-migrate) нь POSTGRES_USER — superuser, CREATE EXTENSION, RLS DDL хийхэд шаардлагатай; api нь APP_DB_USER — NOSUPERUSER NOBYPASSRLS роль, эхний ачаалалд deploy/initdb скриптээр үүсдэг. Байгаа өгөгдлийн сан дээр deploy хийж байгаа бол уг ролийг гараар үүсгэж эрх олгоод APP_DB_DSN-ийг түүн рүү заана.',
 '{deploy,postgres,роль,rls}', 'docs/DEPLOYMENT', 'mn'),

('ops-rollback-backup', 'Буцаах ба нөөцлөлт',
 'Буцаахдаа сүүлийн ажиллаж байсан commit руу шилжээд docker compose build && up -d хийнэ. SQL migration нь зөвхөн урагшаа урсдаг тул migration буцаах шаардлагатай бол харгалзах N_*.down.sql-ийг ГАРААР ажиллуулж байж кодоо буцаана. Нууц түлхүүрийн эргэлт: JWT_SECRET солих нь бүгдийг гаргана, Hydra-гийн нууцууд солигдвол OIDC session/зөвшөөрөл хүчингүй болно.',
 '{rollback,бэкап,migration,ops}', 'docs/DEPLOYMENT', 'mn'),

('config-env-core', 'Үндсэн орчны хувьсагчид',
 'PORT (сонсох порт), ENVIRONMENT (development / production — production-д хатуу шалгалтууд асна), DEBUG, ALLOWED_ORIGINS (CORS-ийн яг таарах жагсаалт, production-д заавал), TRUSTED_PROXIES (прокси хаяг — X-Forwarded-For-т итгэх), JWT_SECRET (≥32 тэмдэгт), JWT_EXPIRED / JWT_REFRESH_EXPIRED, DB_POSTGRE_DSN эсвэл DB_POSTGRE_URL (production-д sslmode=verify-full заавал), REDIS_HOST / REDIS_PASS, OBSERVABILITY_TOKEN.',
 '{тохиргоо,env,хувьсагч}', 'docs-site/configuration', 'mn'),

('config-env-integrations', 'eID, SSO, AI-ийн тохиргоо',
 'eID: EID_BASE_URL (…/v3), EID_RP_UUID, EID_RP_NAME, EID_RP_SECRET, EID_CERT_LEVEL, EID_CALLBACK_URL (IdP дээр цагаан жагсаалтад байх ёстой). Gerege SSO consumer: SSO_ISSUER, SSO_CLIENT_ID, SSO_CLIENT_SECRET, SSO_REDIRECT_URI, SSO_SCOPE. OIDC provider тал: OAUTH_ISSUER, SSO_STATE_KEY (≥32 байт), SSO_FIRSTPARTY_CLIENTS. AI: GEMINI_API_KEY, GEMINI_MODEL, GEMINI_TTS_MODEL, GEMINI_VOICE, AI_SCOPE_PROMPT. Хоосон бол холбогдох боломж идэвхгүй болно (алдаа өгч boot зогсохгүй).',
 '{тохиргоо,eid,sso,gemini,env}', 'docs-site/configuration', 'mn'),

('ops-observability', 'Ажиглалт (observability)',
 'OpenTelemetry trace (OTEL_EXPORTER: хоосон = унтраалттай, stdout, otlp; OTEL_SAMPLE_RATIO), Prometheus metrics (/metrics — HTTP тоологч ба латент, кэшийн hit/miss, pgx pool-ийн бодит статистик), Zap структурт лог (production-д JSON) бөгөөд request id, trace id-г дамжуулна. Нууц утга хэзээ ч лог-д бичигдэхгүй.',
 '{ажиглалт,otel,prometheus,лог}', 'backend/docs/ARCHITECTURE', 'mn'),

-- ── Хөгжүүлэлт ────────────────────────────────────────────────────────────
('dev-add-feature', 'Шинэ боломж хэрхэн нэмэх вэ',
 'Дараалал: (1) domain сущность, (2) repositories/interface-д интерфэйс, (3) records бүтэц + postgres репозитор (гар бичмэл SQL, pgx.RowToStructByName, deleted_at IS NULL, 23505 → Conflict), (4) usecase интерфэйс + хэрэгжүүлэлт, (5) DTO (validate тагтай), (6) handler — func(w,r) error хэлбэртэй, v1.Wrap-аар боогдоно, (7) route бүртгэл, (8) cmd/api/server/server.go дотор repo → usecase → route гар холболт, (9) хэрэглэгчийн өгөгдөл бол RLS бодлого + withRLS.',
 '{хөгжүүлэлт,шинэ feature,заавар}', 'backend/docs/DEVELOPMENT', 'mn'),

('dev-testing', 'Тестийн стратеги',
 'Unit тест — usecase болон handler давхарга, mockery-гээр үүсгэсэн mock-той, Docker шаардахгүй хурдан (go test ./...). Интеграци тест — testcontainers-оор бодит Postgres + Redis дээр репозитор, RLS бодлогуудыг шалгана (make test-integration, Docker хэрэгтэй). Frontend талд vitest (bff, i18n паритет, navigation). Route бүрийн эрхийн шалгуурыг authz matrix тест баталгаажуулна. make pre-push нь CI-г бүрэн давтана: lint + тест + swagger дрейф + build.',
 '{тест,unit,интеграци,ci}', 'backend/docs/DEVELOPMENT', 'mn'),

('dev-migrations', 'Migration бичих',
 'Migration нь backend/migrations доторх дугаарласан SQL файлууд: N_нэр.up.sql болон N_нэр.down.sql. Схемийг ЗӨВХӨН эдгээр файл тодорхойлно — ORM AutoMigrate байхгүй, records бүтэц нь зөвхөн pgx-ийн скан хийх энгийн struct. Гүйцэтгэгч файлуудыг дугаараар эрэмбэлж, файл бүрийг schema_migrations мөртэй нь нэг гүйлгээнд хийж, бүх ажиллагаанд session advisory lock барина (зэрэг ажиллуулахад дараалдана).',
 '{migration,sql,схем}', 'backend/docs/DEVELOPMENT', 'mn'),

('dev-conventions', 'Кодын конвенц',
 'Код доторх танигч ба commit мессеж англиар; комментууд болон интерфейсийн мөрүүд монголоор. Файл бүр хоёр мөрт Gerege Template Platform V3.0 толгойтой. Commit нь conventional commits (feat:, fix:, docs:, chore:). Баримт нь EN/MN/ZH/RU багцаар — нэгийг засвал бусдыг нь мөн; i18n түлхүүр дөрвөн хэлэнд байх ёстой (тест шалгана). Handler-ийн swagger аннотац өөрчлөгдвөл make swag ажиллуулж docs/-ийг commit хийнэ, эс бөгөөс CI унана.',
 '{конвенц,commit,баримт,swagger}', 'CLAUDE.md', 'mn'),

('api-response-format', 'API-ийн хариу хэлбэр',
 'Бүх хариу нэг дугтуйтай: { status, message, data, request_id }. Амжилтад status=true ба data байна; алдаанд status=false. Баталгаажуулалтын алдаа 422 бөгөөд data.errors дотор талбар тус бүрийн { field, tag, message } ирнэ. Домэйн алдаанууд статус болж хөрвөнө: NotFound→404, Unauthorized→401, Forbidden→403, Conflict→409, BadRequest→400, Internal→500. 5xx-ийн жинхэнэ шалтгаан зөвхөн лог-д бичигдэж, хэрэглэгчид ерөнхий мессеж очно. request_id нь X-Request-ID толгойд ч давхардана.',
 '{api,хариу,алдаа,статус}', 'backend/docs/API_CONTRACT', 'mn')

ON CONFLICT (slug) DO UPDATE SET
    title      = EXCLUDED.title,
    content    = EXCLUDED.content,
    tags       = EXCLUDED.tags,
    source     = EXCLUDED.source,
    lang       = EXCLUDED.lang,
    updated_at = now(),
    -- Агуулга өөрчлөгдсөн бол embedding-ийг хүчингүй болгоно — backfill дахин
    -- тооцоолж, хайлт хуучин вектороор андуурахгүй.
    embedding    = CASE WHEN ai_knowledge.content IS DISTINCT FROM EXCLUDED.content
                        THEN NULL ELSE ai_knowledge.embedding END,
    content_hash = CASE WHEN ai_knowledge.content IS DISTINCT FROM EXCLUDED.content
                        THEN NULL ELSE ai_knowledge.content_hash END;

-- Гараар id өгсөн хуучин seed-тэй зөрчилдөхгүйн тулд sequence-ийг гүйцээнэ.
SELECT setval('ai_knowledge_id_seq', GREATEST((SELECT MAX(id) FROM ai_knowledge), 1));
