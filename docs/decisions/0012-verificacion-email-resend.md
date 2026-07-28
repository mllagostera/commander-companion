# ADR-0012: Verificación de email en el registro con Resend

**Estado:** Aceptada e implementada (2026-07-28)

## Contexto

`POST /auth/register` creaba la cuenta con cualquier email, sin comprobar que
quien se registra sea dueño de esa casilla, y el BFF de Nuxt encadenaba un
login automático que dejaba la sesión iniciada de una. No había ninguna
librería de envío de mail en el backend.

Hacía falta: (1) confirmar el email antes de dejar operar la cuenta por
password, igual que cualquier registro estándar, y (2) un proveedor de mail
transaccional cuyos mails no terminen en spam.

## Decisión

### Modelo de datos

- `users.email_verified boolean NOT NULL DEFAULT true` (migración
  `00011_email_verification.sql`). Default `true` para no tener que migrar
  usuarios existentes ni tocar `CreateUserWithGoogle`: Google ya confirma el
  email en su propio `id_token` (ver [ADR-0002](0002-google-sign-in.md)), así
  que solo el alta por email/password (`CreateUser`) fuerza `false`
  explícitamente.
- `email_verification_tokens`, mismo patrón que `refresh_tokens`
  ([ADR-0001](0001-auth-jwt-refresh-token-strategy.md)): solo se persiste el
  hash SHA-256 del token, nunca el valor en claro; TTL de 24h; uso único
  (`used_at`).
- `LinkGoogleID` (vincular una cuenta de Google a una cuenta email/password
  existente) también marca `email_verified = true`: en ese punto
  `FindOrCreateGoogleUser` ya comprobó que Google confirma ese email, así que
  una cuenta todavía no confirmada queda verificada por esa vía también.

### Política de login: bloqueado hasta verificar

`users.VerifyCredentials` (de la que depende `auth.Login`) devuelve
`ErrEmailNotConfirmed` (`403`) si el password es correcto pero
`email_verified` es `false`. Es un código distinto de `ErrInvalidCredentials`
(`401`) porque acá ya se probó que la contraseña es correcta — el cliente usa
el `403` para ofrecer "reenviar verificación" en vez de un genérico "revisá
tus credenciales".

Se consideró la alternativa de dejar entrar con un banner "verificá tu
email" y restringir alguna acción puntual, pero no hay ninguna acción hoy
que tenga sentido restringir a medias, y bloquear el login es el patrón más
simple de razonar sobre el modelo de JWT actual.

`web/server/api/auth/register.post.ts` deja de encadenar un login automático
tras registrar (el login fallaría con `403` igual): la pantalla de registro
muestra un "revisá tu email" en vez de navegar al dashboard.

### Flag `REQUIRE_EMAIL_VERIFICATION` (default `false` en fase alpha)

El proyecto está en fase alpha: no tiene sentido ni gastar el envío de mail
ni bloquear el login por esto todavía. `config.Config.RequireEmailVerification`
(env var `REQUIRE_EMAIL_VERIFICATION`, default `false`) controla todo el
comportamiento de arriba desde `users.NewService`:

- En `false` (default), `RegisterUser` crea la cuenta con `email_verified =
  true` de entrada y **ni genera el token ni llama al `Mailer`** — no
  paga el costo de un envío que nadie va a exigir. `VerifyCredentials` no
  necesita ningún caso especial: la cuenta ya está verificada, así que el
  login anda de una.
- En `true`, es el flujo completo descripto arriba (token, mail, `403` hasta
  confirmar).

Cuando el proyecto salga de alpha, se prende con `REQUIRE_EMAIL_VERIFICATION=true`
sin tocar código.

### Proveedor: Resend, con templates de dashboard (no HTML en el backend)

Se eligió **Resend** por su API HTTP simple y buen deliverability por
defecto para un proyecto de este tamaño.

El contenido del mail (asunto, copy, layout) vive en un **Template** del
dashboard de Resend, no en el backend: `internal/email` solo llama a
`POST https://api.resend.com/emails` con `template: { id, variables }`
(`USERNAME`, `VERIFY_URL`), sin una línea de HTML/texto en Go.

**Salvedad, verificada contra un issue abierto de Resend**
([resend/react-email#3247](https://github.com/resend/react-email/issues/3247),
sin fix a la fecha de esta decisión): mandar por la REST API un template
cuya variable esté dentro de un `href` (botón/link) rompe la URL en el envío
real — el botón "Send test" del dashboard no reproduce el bug, así que
engaña. Por eso el template debe mostrar `VERIFY_URL` como **texto plano
visible**, no como botón: la mayoría de los clientes de mail auto-linkean
URLs sueltas igual, así que el link sigue siendo cliqueable. Si Resend
arregla el bug más adelante, se puede volver a un botón sin tocar el
backend — es un cambio solo del template.

### Modo consola sin cuenta de Resend

`email.NewResendClient` devuelve un mailer que solo hace `log.Printf` del
link de verificación cuando `RESEND_API_KEY` está vacío. Así
`docker-compose up` (y los tests) siguen funcionando sin que nadie necesite
una cuenta de Resend para desarrollar en local.

## Alternativas consideradas

- **SendGrid / Amazon SES**: descartados por preferencia del usuario del
  proyecto (cuenta de Resend ya existente) y porque SES requiere más
  configuración manual de dominio/reputación para salir de modo sandbox.
- **HTML armado en el backend** (`html`/`text` directos en el POST a
  Resend, sin `template`): evita por completo el bug de `href` mencionado
  arriba, pero implica versionar el copy del mail en el repo en vez del
  dashboard de Resend, que es donde el usuario del proyecto prefiere
  mantenerlo. Se descartó a favor de templates de dashboard + link como
  texto plano.
- **Límite de reenvío por email** (además del rate limit por IP existente):
  se descartó por alcance — el rate limit por IP de 20 req/min que ya
  protege todos los endpoints públicos de auth
  (`cmd/api/main.go: newAuthRateLimiter`) alcanza para este cambio; no hay
  hoy un caso de abuso documentado que lo justifique.

## Consecuencias

- **Paso manual pendiente, fuera del código**: para que los mails no caigan
  en spam hace falta verificar un dominio propio en el dashboard de Resend
  (genera los registros SPF/DKIM/DMARC a agregar en el DNS del dominio), y
  crear ahí el Template de verificación (variables `USERNAME` y
  `VERIFY_URL`, publicado, `VERIFY_URL` como texto plano) — ver
  `docs/roadmap/TASKS.md`, sección Auth — verificación de email.
- Sin `RESEND_API_KEY`/`EMAIL_FROM`/`RESEND_VERIFY_EMAIL_TEMPLATE_ID`
  configurados, el registro sigue funcionando end-to-end en local (modo
  consola), pero no manda mails reales.
- Con `REQUIRE_EMAIL_VERIFICATION=false` (default), toda esta feature queda
  construida pero inactiva: la columna, la tabla de tokens y los endpoints
  `/auth/verify-email`/`/auth/resend-verification` siguen ahí, pero ninguna
  cuenta nueva los necesita. Hay que acordarse de prender el flag (y de tener
  el dominio/template de Resend listos) antes de salir de alpha — queda
  anotado en `docs/roadmap/TASKS.md`.

## Referencias

- Fuente de referencia del Template de Resend (se pega tal cual en el dashboard,
  Templates → Create template → From code): `0012-verify-email-template.html`
- Implementación: `backend/internal/email/resend.go`,
  `backend/internal/users/service.go` (`RegisterUser`, `VerifyEmail`,
  `ResendVerification`)
- Migración: `backend/migrations/00011_email_verification.sql`
- Ver también [ADR-0001](0001-auth-jwt-refresh-token-strategy.md) (mismo
  patrón de token opaco + hash SHA-256) y
  [ADR-0002](0002-google-sign-in.md) (por qué las cuentas de Google ya
  quedan verificadas de alta)
