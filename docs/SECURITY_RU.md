# Состояние безопасности — Gerege Template Platform V3.0

> 🌐 **Русский** · [English](SECURITY.md) · [中文](SECURITY_ZH.md) · пояснения на
> монгольском см. в комментариях кода. Порядок сообщения об уязвимостях —
> в [`/SECURITY.md`](../../SECURITY.md).

Этот документ сопоставляет реализованные в бэкенде меры со стандартом
безопасности проекта — на основе **OWASP ASVS / API Top 10, NIST SP 800-63B /
800-218 и CIS Controls**. Здесь зафиксировано, что обеспечивается кодом, что было
усилено и что остаётся на будущие этапы. Чтобы сообщить об уязвимости, см.
[политику безопасности](../../SECURITY.md) репозитория.

> **Модель аутентификации.** Единственный интерактивный вход — **eID (доверяющая
> сторона eID Mongolia)**: привязка устройства / QR + push по регистрационному
> номеру с long-poll-сессией, плюс привязка аккаунта через **Google OAuth**.
> **Ни один маршрут не подключает вход по паролю или email-OTP**; устаревшие
> usecase `Login` / `Register` / OTP / сброс пароля существуют в дереве кода, но
> не публикуются в `route_auth.go` (там регистрируются только eID / Google /
> refresh / logout). Меры ниже отражают реальную поверхность.

## Реализованные меры (в коде)

| Область | Мера | Где | § руководства |
|------|---------|-------|---------|
| Аутентификация | JWT access+refresh, ротация, защита claim `kind` | `pkg/jwt`, `usecases/auth` | §1.3–1.4 |
| Аутентификация | Вход через eID (RP Монголии) — привязка устройства / QR + push по регистрационному номеру, long-poll-сессия; **единственный** интерактивный вход | `usecases/auth/auth_eid.go`, `pkg/eid`, `routes/route_auth.go` | §1 |
| Аутентификация | Сертификат гражданина eID (PKI) — при завершении входа возвращается сертификат (DER); разбирается через `crypto/x509`, серийный номер / срок действия / издатель / тип ключа сохраняются | `pkg/eid`, `migrations/16_users_eid_certificate.up.sql` | §1 |
| Аутентификация | Федеративная личность — привязка аккаунта Google OAuth по стабильному столбцу subject | `usecases/auth`, `migrations/18_users_google_sub` | §1 |
| Криптография | `crypto/rand` для идентификаторов токенов и сессий; отбраковка значений исключает смещение по модулю | `pkg/helpers` | §13.2 |
| Криптография | Интеграционные токены шифруются при хранении — OAuth-токены сторонних сервисов запечатываются **AES-256-GCM**; ключ из `INTEGRATION_ENC_KEY` | `usecases/integrations/integrations_crypto.go`, `migrations/21_user_integrations` | §7.3 |
| Аудит | Журнал аудита с хеш-цепочкой, только на добавление — `chain_hash = SHA-256(prev_hash ‖ canonical-json(entry))`, записи сериализуются через `pg_advisory_xact_lock`; `VerifyChain` выявляет подделку | `pkg/audit/chain.go`, `usecases/audit`, `migrations/15_audit_log` | §9 |
| Авторизация | Динамический RBAC (роли + разрешения), SuperAdmin/Admin/Manager/User; middleware `RequirePermission` / `RequireAdmin`; админ автоматически получает полный каталог разрешений | `middleware_rbac.go`, `domain_users.go`, `migrations/8_rbac_roles_permissions`, `migrations/23_superadmin_role` | §2 |
| Авторизация | Поверхность **провайдера** OIDC — платформа сама является провайдером OIDC (ядро вход/согласие/выход, встроенный Go-провайдер); согласие определяет, какие claims гражданина отдаются на каждый scope; реестр RP `developer_apps` — источник истины о владении клиентами | `usecases/provider`, `usecases/oidc`, `postgres/oauth`, `migrations/42_oauth_provider` | §2 |
| БД | Только параметризованные запросы (pgx) | `datasources/repositories/postgres` | §3.1 |
| БД | `INSERT … RETURNING` за один round-trip; pgconn 23505 → Conflict | `repositories/postgres/users`, `driver_pgx.go` | §3 |
| БД | Row-Level Security на каждой пользовательской таблице (ENABLE + **FORCE**): `users`, а также `organizations` / `organization_memberships`, гражданские таблицы `gov_*` и `user_integrations` — политики self/admin/service на GUC `app.user_id`/`app.user_role`, задаваемых в транзакции через `SET LOCAL` внутри `withRLS`; нет личности ⇒ ноль строк (fail-closed) | `migrations/7_enable_rls_users`, `migrations/14`, `migrations/20`, `migrations/21`, `datasources/rls`, `repositories/postgres/*` | §2.4/§3.3 |
| API | Защита от mass assignment (явные request-DTO) | `http/datatransfers/requests` | API3 §5.1 |
| API | Ограничение размера тела (общее + 4 КиБ на `/auth`) | `middleware.bodysizelimit`, `routes` | §5.3 |
| Web | Заголовки безопасности: CSP `default-src 'none'`, HSTS (прод), nosniff, X-Frame DENY, Referrer-Policy, Permissions-Policy, COOP/CORP/COEP | `middleware_security.go` | §4.7 |
| Web | Строгий список origin для CORS, никогда `*`+credentials | `middleware.cors.go` | §4.8 |
| Эксплуатация | Операторские endpoint (`/metrics`, `/swagger/doc.json`) закрыты в проде: bearer-токен (сравнение за постоянное время) + 404 при промахе | `middleware_observability_gate.go`, `cmd/api/server` | §4.7/§9 |
| Наблюдаемость | Структурированные логи Zap с request-id; секреты не логируются | `pkg/logger`, `handler_base_response.go` | §9.1–9.2 |
| Наблюдаемость | Трассировка OpenTelemetry + метрики Prometheus | `pkg/observability`, `driver_pgx.go` | §9.4 |
| Эксплуатация | Корректное завершение (HTTP, лимитеры, пул pgx, Redis, tracer) | `cmd/api/server` | §7 |
| Сеть | Полные таймауты HTTP-сервера (`ReadHeader` 10 с, `Read` 30 с, `Write` 60 с, `Idle` 120 с) + `MaxHeaderBytes` 16 КиБ — защита от slowloris и огромных заголовков | `cmd/api/server` | §5.3 / API4 |
| Аутентификация | Список запрещённых access-токенов при выходе — `jti` попадает в Redis на остаток TTL; middleware отклоняет их на каждом запросе | `usecases/auth.logout`, `middleware_auth.go` | §1.4 |
| БД | Проверка RLS при старте — приложение смотрит свою роль в БД; суперпользователь / `BYPASSRLS` прерывает старт в продакшене (иначе RLS молча не применялся бы), в разработке — предупреждение | `datasources/drivers/driver_pgx.go` | §2.4/§3.4 |
| AI | Слоистый системный промпт: жёстко заданные ограничения (соблюдение области, устойчивость к prompt injection, запрет раскрывать промпт) + настраиваемые из базы scope/instructions; `SetPrompt` только UPDATE по засеянным ключам | `usecases/ai/ai_prompts.go`, `migrations/11` | §5.1 |
| AI | Гигиена ввода AI: whitelist mime для аудио и лимит ~700 КБ base64, ограничения длины сообщения и истории, отдельный лимит `/ai` (~20/мин), ошибки инструментов возвращаются модели — никогда клиенту | `requests_ai.go`, `routes/route_ai.go` | §5.1/§5.3 |

## Проведённое усиление (этот проход — по руководству)

1. **Заголовки cross-origin изоляции** — добавлены
   `Cross-Origin-Opener-Policy: same-origin`, `Cross-Origin-Resource-Policy: same-site`,
   `Cross-Origin-Embedder-Policy: require-corp` в `middleware.security.go`
   (руководство §4.6/4.7). *Проверено на работающем сервере.*
2. **Проверка TLS для продакшен-БД** — валидация конфигурации отклоняет
   продакшен-`DB_POSTGRE_URL` без `sslmode=verify-full` (или `verify-ca`);
   это задокументировано в `.env.example` (`internal/config/config.go`, §3.5).
3. **Таймаут на запрос** — `middleware.TimeoutMiddleware` ставит дедлайн контекста
   30 с, который передаётся в запросы pgx и ограничивает зависшие обработчики
   (`middleware.timeout.go`, §5.3 / API4). Единственное исключение —
   `/api/v1/ai/*` с 50 с (`AIRequestTimeout`): TTS/STT в Gemini обычно занимает
   10–20 с, и предел в 30 с превращал обычные вызовы в 500. Значение остаётся
   ниже 60-секундного read-таймаута обратного прокси, а `Write`-таймаут
   HTTP-сервера выводится из него.
4. **Swagger-спека отдаётся из сгенерированного пакета `docs`** — OpenAPI JSON
   доступен по `/swagger/doc.json` из сгенерированного пакета на роутере chi
   (без Fiber); на него можно направить статический Swagger UI.
5. **Закрытие операторских endpoint** — `/metrics` и `/swagger/doc.json` больше не
   публичны. В продакшене `ObservabilityGate` требует
   `Authorization: Bearer <OBSERVABILITY_TOKEN>` (сравнение
   `crypto/subtle.ConstantTimeCompare`) и возвращает **404** (а не 401) при любом
   промахе, скрывая сами endpoint от разведки. Пустой токен ⇒ полностью закрыто.
   `/health` и `/ready` остаются публичными для балансировщиков.
6. **RLS в Postgres + разделение ролей БД** — у `users` теперь RLS
   **ENABLE + FORCE** с политиками self/admin/service. Личность запроса попадает в
   каждый запрос из контекста через `SET LOCAL app.user_id`/`app.user_role` внутри
   транзакции `withRLS` репозитория; нет личности ⇒ ноль строк (fail-closed).
   Контейнер `api` в compose подключается ролью без прав суперпользователя
   `APP_DB_USER` (создаётся скриптом `deploy/initdb/10-create-app-user.sh`), чтобы
   политики действительно работали; `migrate` сохраняет суперпользователя для DDL.
   Подтверждено интеграционным тестом, подключающимся ролью без суперправ
   (`users_rls_test.go`).
7. **Усиление HTTP-сервера** — помимо `ReadHeaderTimeout` заданы
   `ReadTimeout`/`WriteTimeout`/`IdleTimeout`, заголовки ограничены 16 КиБ;
   `WriteTimeout` выводится из бюджета таймаута запроса (×2), чтобы сервер не
   обрывал обработчики первым.
8. **Выход отзывает оба токена** — `jti` refresh-токена удаляется (как и раньше),
   а `jti` access-токена помещается в список запрещённых Redis на оставшееся время
   жизни; middleware аутентификации проверяет список на каждом запросе
   (fail-open при ошибках Redis — та же политика, что и для отсечки по смене пароля).
9. **Проверка применимости RLS при старте** — приложение запрашивает `pg_roles`
   для своей роли; суперпользователь или `BYPASSRLS` прерывает старт в продакшене
   и пишет предупреждение в разработке, поэтому неверно заданный DSN больше не
   может молча отключить RLS.
10. **Ограничения AI** — ассистент Gemini работает на слоистом промпте, базовый
    слой которого (только монгольский, соблюдение области, устойчивость к prompt
    injection) жёстко задан; админом редактируются только слои scope/instructions
    (`settings.manage`, только UPDATE по засеянным ключам). Инструменты
    выполняются на сервере с контекстом запроса; их сбои сообщаются модели как
    данные и никогда не утекают клиенту.

## Статус дорожной карты ASVS (§14 руководства)

- **Этап 1 (ASVS L1):** ✅ готовность к HTTPS + HSTS, вход только через eID (нет
  парольной поверхности), параметризованные запросы, заголовки безопасности,
  строгий CORS, валидация ввода, структурированные логи, `.gitignore` и отсутствие
  секретов в репозитории. ⏳ сканирование контейнеров / `govulncheck` в CI (`.github/`).
- **Этап 2 (ASVS L2):** ✅ ограничение запросов, ротация refresh-токенов,
  аутентификация через привязку устройства eID (устойчива к фишингу, личность на
  аппаратной основе), таймаут запроса, шифрование интеграционных токенов, аудит с
  хеш-цепочкой. ⏳ WAF, централизованный SIEM, тест восстановления из
  зашифрованного бэкапа, план реагирования на инциденты.
- **Этап 3 (ASVS L3):** ◻ шифрование PII на уровне полей (KMS), mTLS, провенанс
  SLSA L3, внешний пентест. (Вне рамок шаблона.)

## Известные пробелы и задачи

- **Интерактивный Swagger UI** — сейчас отдаётся только сырая спецификация по
  `/swagger/doc.json` (загрузите в Swagger Editor / Postman или направьте на неё
  статический Swagger UI).
- **Парольные меры (HIBP / bcrypt / утёкшие пароли)** — **неприменимы к текущей
  поверхности**: парольный вход не подключён (аутентификация — eID + Google OAuth).
  Устаревшие usecase с паролями и OTP остались в дереве, но недостижимы; если
  парольный путь снова откроют, подключите проверку утёкших паролей
  (HIBP k-anonymity, §1.1) до релиза.
- **RLS в Postgres** (§2.4/§3.3) — ✅ включён **и FORCED** на каждой
  пользовательской таблице (`users`, `organizations` / `organization_memberships`,
  гражданские таблицы `gov_*`, `user_integrations`) с политиками self/admin/service
  на сессионных GUC `app.user_id`/`app.user_role` (`SET LOCAL` в `withRLS` каждого
  репозитория). Это эшелонированная защита поверх условий `deleted_at IS NULL` /
  WHERE, которые уже пишут репозитории; запрос без личности fail-closed. Публичные
  справочные таблицы (например, каталог `gov_services`) остаются без RLS и
  опираются на табличные привилегии. Для **мультитенантности** добавьте столбец
  `tenant_id` и политику арендатора в каждую таблицу и передавайте арендатора в
  `rls.Identity`.
- **Менеджер секретов / KMS** (§7.3) — в продакшене используйте настоящее
  хранилище секретов; `.env` — только для локальной разработки и в gitignore.
- **Разделение ролей БД** (§3.4) — ✅ **встроено в compose-стек** (это обязательно:
  RLS, даже FORCED, обходится суперпользователями и ролями BYPASSRLS, а образ
  postgres делает `POSTGRES_USER` суперпользователем). При первой инициализации БД
  `deploy/initdb/10-create-app-user.sh` создаёт роль **без суперправ**
  `APP_DB_USER` (`NOSUPERUSER NOBYPASSRLS`) и выдаёт ей DML через привилегии по
  умолчанию. **api** подключается этой ролью (compose переопределяет
  `DB_POSTGRE_DSN` значением `APP_DB_DSN` — стек работает в режиме development,
  поэтому драйвер читает DSN в keyword-формате), и RLS работает; контейнер
  **migrate** продолжает использовать `POSTGRES_USER` (для
  `CREATE EXTENSION "uuid-ossp"` и DDL RLS нужны суперправа).
  Быстрая проверка из подключения api:
  `SELECT rolsuper, rolbypassrls FROM pg_roles WHERE rolname = current_user;` —
  оба значения должны быть `false`. Если `APP_DB_URL` оставить на
  суперпользователе, RLS *не* работает (он молча становится no-op).

---

**Gerege Template Platform V3.0** — совместная разработка **команды Gerege Systems** и **Claude AI**, 2026.
