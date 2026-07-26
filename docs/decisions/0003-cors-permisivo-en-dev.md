# ADR-0003: CORS abierto por defecto, restringible por variable de entorno

**Estado:** Aceptada e implementada (2026-07-26)

## Contexto

El backend no tenía middleware de CORS. Mientras el único cliente fue Android
(que no está sujeto a same-origin policy) esto no era un problema, pero dejó
de serlo en cuanto apareció el primer cliente basado en navegador:
`tools/auth-test/` (herramienta de test manual del flujo de auth) y, a
futuro, el cliente Nuxt (ver [ADR-0004](0004-web-client-nuxt.md)).

## Decisión

Se agrega `github.com/gofiber/fiber/v2/middleware/cors`, con los orígenes
permitidos leídos de `CORS_ALLOWED_ORIGINS` (lista separada por comas). Si la
variable está vacía, **por defecto se permite cualquier origen (`*`)**.

Esto es seguro porque la API **nunca usa cookies** para autenticación — todo
es Bearer token en el header `Authorization`, que un origen malicioso no
puede leer ni adjuntar automáticamente a requests cross-site como sí pasaría
con cookies. Un `Access-Control-Allow-Origin: *` no expone nada que un
atacante no pudiera obtener igual haciendo el request server-to-server
(la API ya está pensada para ser pública/consumida por múltiples clientes).

## Alternativas consideradas

- **Lista blanca obligatoria desde el día 1**: más "seguro" en apariencia,
  pero en la práctica solo agrega fricción de configuración en dev/testing
  (cada nuevo puerto de servidor estático local requeriría tocar la env var)
  sin mitigar un riesgo real, dado que no hay cookies de por medio.
- **Reflejar el `Origin` recibido dinámicamamente sin lista**: equivalente en
  la práctica a `*` para este caso (sin credentials), pero más código para el
  mismo resultado.

## Consecuencias

- En cualquier entorno que no sea desarrollo local, hay que setear
  `CORS_ALLOWED_ORIGINS` explícitamente a los orígenes reales (el dominio del
  cliente Nuxt en producción, etc.) — el default abierto es intencionalmente
  solo para dev.
- Si en el futuro se agrega autenticación basada en cookies (por ejemplo, un
  refresh token en una cookie `HttpOnly` para el cliente web), esta decisión
  hay que revisarla: `AllowOrigins: "*"` es incompatible con
  `AllowCredentials: true` por especificación CORS, y en ese escenario sí
  haría falta una lista blanca real.

## Referencias

- Implementación: `backend/cmd/api/main.go` (`corsAllowedOrigins`)
- `backend/.env.example`
