# ADR-0002: Google Sign-In como proveedor adicional de autenticación

**Estado:** Aceptada e implementada (2026-07-26)

## Contexto

El roadmap original preveía email/password como único método de login. Se
decidió agregar "Sign in with Google" como proveedor adicional (no como
reemplazo) para reducir fricción de registro, especialmente en Android donde
Credential Manager + Google Identity Services es el flujo recomendado por
Google.

Esto obliga a decidir: cómo se relaciona una cuenta de Google con una cuenta
existente por email/password, y con qué librería se verifica el `id_token`
en el backend.

## Decisión

### Modelo de cuenta

- `users.password_hash` pasa a **nullable**; `users.google_id` (varchar,
  único, nullable) se agrega. `CHECK (password_hash IS NOT NULL OR
  google_id IS NOT NULL)` a nivel de base — un usuario siempre tiene *al
  menos* una forma de autenticarse, nunca ninguna.
- Flujo de `POST /auth/google` (`users.FindOrCreateGoogleUser`):
  1. Busca por `google_id`. Si existe, login directo.
  2. Si no, busca por `email`. Si existe una cuenta con ese email **y**
     Google confirma `email_verified: true` en el `id_token`, se **vincula
     automáticamente** (`LinkGoogleID`) — no se pide confirmación adicional.
  3. Si tampoco existe, se crea una cuenta nueva sin password, con un
     username derivado del local-part del email (con sufijo si hay colisión).
  4. Si Google no confirma `email_verified`, se rechaza (`400`) antes de
     buscar/crear nada.

### Librería de verificación del `id_token`

Se usa **`github.com/coreos/go-oidc/v3`** en vez de la librería oficial
`google.golang.org/api/idtoken`.

## Alternativas consideradas

- **Exigir confirmación manual al vincular por email**: más seguro ante el
  caso borde de un email verificado por Google pero que en la práctica no
  controla el dueño de la cuenta password (extremadamente raro, ya que
  Google solo marca `email_verified: true` tras su propio flujo de
  verificación). Se descartó por ahora para no agregar un paso de UX extra;
  documentado como simplificación consciente, no como olvido.
- **`google.golang.org/api/idtoken`** (librería oficial de Google): hace
  exactamente lo mismo (discovery + JWKS + validación de issuer/audience/
  firma), pero arrastra todo el módulo `google.golang.org/api`
  (`cloud.google.com/go/auth`, gRPC, OpenTelemetry, etc.) — un árbol de
  dependencias enorme para verificar un token. Confirmado con `go get`: sumaba
  ~20 paquetes indirectos nuevos. Se descartó por peso/superficie de ataque
  desproporcionados frente al problema real.
- **JWKS manual** (`golang-jwt/jwt` + fetch propio del JWKS de Google): la
  opción más liviana en teoría, pero reimplementa caching/rotación de claves
  que `go-oidc` ya resuelve correctamente (es la librería estándar de la
  comunidad Go para verificación OIDC, con muchas menos dependencias que la
  opción oficial de Google).

## Consecuencias

- El discovery document de Google (`https://accounts.google.com/.well-known/
  openid-configuration` y su JWKS) se resuelve **perezosamente**, en el primer
  login con Google, no al arrancar el servidor — el arranque no depende de
  que Google esté alcanzable.
- Sin `GOOGLE_CLIENT_ID` configurado, `POST /auth/google` responde `501` en
  vez de fallar el arranque o crashear en runtime.
- Crear las credenciales OAuth reales (Web Client ID + Android Client ID) en
  Google Cloud Console es un paso manual externo que no se puede automatizar
  desde el repo — ver `docs/roadmap/TASKS.md`, sección Auth — Google OAuth.

## Referencias

- Implementación: `backend/internal/auth/google.go`,
  `backend/internal/users/service.go` (`FindOrCreateGoogleUser`)
- Migración: `backend/migrations/00002_auth.sql`
- Ver también [ADR-0001](0001-auth-jwt-refresh-token-strategy.md) (mismo par
  de tokens se emite para login por password o por Google)
