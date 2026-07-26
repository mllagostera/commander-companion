# ADR-0001: Estrategia de autenticación — JWT de acceso + refresh token opaco rotativo

**Estado:** Aceptada e implementada (2026-07-26)

## Contexto

El backend necesitaba autenticación real (hasta este punto, `internal/auth/`
estaba vacío y `internal/users/service.go` usaba un hash de contraseña dummy).
Había que decidir:

1. Cómo firmar los access tokens: secreto simétrico (HS256) vs. par de claves
   asimétrico (RS256/ES256).
2. Cómo manejar la renovación de sesión (`POST /auth/refresh`) y el logout
   real (`POST /auth/logout` debe poder invalidar una sesión).
3. Formato del refresh token: JWT también, u opaco.

## Decisión

- **Access token: JWT firmado con HS256**, secreto simétrico (`JWT_SECRET`),
  vida corta (`ACCESS_TOKEN_TTL`, default 15 minutos). Claims mínimos:
  `sub` (user ID), `iat`, `exp` (`jwt.RegisteredClaims` de
  `github.com/golang-jwt/jwt/v5`).
- **Refresh token: string opaco aleatorio** (32 bytes de `crypto/rand`,
  base64url), **no un JWT**. Se persiste únicamente su hash SHA-256 en la
  tabla `refresh_tokens` (`token_hash`, `expires_at`, `revoked_at`); el valor
  en claro solo existe en la respuesta HTTP, nunca en la base de datos.
- **Rotación en cada uso**: `POST /auth/refresh` revoca el refresh token
  usado y emite uno nuevo junto con el access token nuevo. Reutilizar un
  refresh token ya rotado devuelve `401`.
- **Logout real**: revoca el refresh token indicado (`revoked_at`), no
  depende de que el access token expire para terminar la sesión.
- Vida del refresh token: `REFRESH_TOKEN_TTL`, default 720h (30 días).

## Alternativas consideradas

- **RS256/ES256** (par de claves): permite que servicios externos verifiquen
  tokens sin conocer un secreto compartido. Se descartó porque el proyecto es
  un monolito modular sin otros servicios que necesiten verificar tokens de
  forma independiente — la complejidad de gestionar un par de claves (rotación,
  distribución de la pública) no se paga con ningún beneficio real hoy.
- **Refresh token también como JWT**: más simple de generar, pero no se puede
  revocar sin una lista de revocación de todas formas — si hay que mantener
  estado en la base para poder revocar, es más directo que el refresh token
  *sea* directamente el puntero a ese estado (opaco + hash), en vez de un JWT
  cuyo contenido nunca se usa una vez que hay que ir a la base igual.
- **Sesiones basadas en cookies**: se descartó porque el cliente principal es
  una app Android nativa (no un navegador con cookies same-site triviales), y
  el segundo cliente (ver [ADR-0004](0004-web-client-nuxt.md)) es un SPA/SSR
  desacoplado que de todas formas usa Bearer tokens.

## Consecuencias

- Revocar *todas* las sesiones de un usuario (ej. "cerrar sesión en todos los
  dispositivos") requiere revocar cada `refresh_tokens.user_id` — no
  implementado todavía, pero la tabla ya soporta la query.
- Comprometer `JWT_SECRET` invalida la confianza en *todos* los access tokens
  emitidos hasta que se rote el secreto (y con HS256 no hay forma de rotar
  sin invalidar tokens viejos — no hay `kid`/multi-secreto implementado).
- El valor por defecto de `JWT_SECRET` en desarrollo
  (`dev-insecure-jwt-secret-change-me`, ver `backend/cmd/api/main.go`) es
  intencionalmente inseguro y debe sobreescribirse en cualquier entorno real.

## Referencias

- Implementación: `backend/internal/auth/token.go`, `service.go`
- Migración: `backend/migrations/00002_auth.sql`
