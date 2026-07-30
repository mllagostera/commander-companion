# ADR-0015: Infraestructura de despliegue — Render + Vercel + Supabase

**Estado:** Aceptada (2026-07-30)

## Contexto

El proyecto corría exclusivamente vía `docker-compose.yml` local; no había ningún
entorno real desplegado. [ROADMAP.md](../roadmap/ROADMAP.md#infraestructura-de-despliegue)
proponía como Opción 1 (recomendada para MVP rápido y coste cero) una PaaS
moderna/serverless: frontend en Vercel o Cloudflare Pages, backend en Fly.io o
Render, base de datos en Neon o Supabase — pero quedaba pendiente de decisión,
sin ADR.

El PR de migraciones automáticas (`internal/common/migrate.go`, ver
`docs/roadmap/TASKS.md`, Stage 1 / Infra) ya se construyó pensando
específicamente en el free tier de Render (que no ofrece un hook de
pre-deploy/release command), y `backend/README.md` ya documenta las env vars
necesarias para Render + Supabase + Vercel. Este ADR formaliza esa decisión,
que ya estaba tomada de facto en el código pero no registrada.

## Decisión

Dentro de la Opción 1 del ROADMAP, se elige:

- **Backend:** [Render](https://render.com), como servicio web Docker (usa el
  `backend/Dockerfile` existente, que ya incluye `migrations/` para que el
  binario las aplique al arrancar). El free tier no ofrece un "Pre-Deploy
  Command" separado; por eso las migraciones corren embebidas en el propio
  binario (`common.RunMigrations`, `cmd/api/main.go`) en vez de depender de
  ese hook.
- **Frontend:** [Vercel](https://vercel.com), para el cliente Nuxt (`web/`).
  Nuxt/Nitro autodetecta el preset de Vercel en build sin configuración
  explícita adicional en `nuxt.config.ts` — no se necesita un `vercel.json`
  para el caso estándar.
- **Base de datos:** [Supabase](https://supabase.com) (PostgreSQL
  administrado). Usar el **Session pooler**, no el Transaction pooler: el
  backend usa `pgx` con prepared statements, que el Transaction pooler no
  soporta correctamente.

Variables de entorno mínimas a configurar en Render (ver
[`backend/README.md`](../../backend/README.md#despliegue-render) para el
detalle completo): `DB_URL` (connection string del Session pooler de
Supabase), `JWT_SECRET` (uno nuevo, no el default de dev), `GOOGLE_CLIENT_ID`,
`CORS_ALLOWED_ORIGINS`, `WEB_APP_URL` (dominio del frontend en Vercel).

### Límite conocido

Si el backend llegara a correr con más de una réplica en Render, todas las
instancias ejecutarían `goose up` en paralelo al bootear. Goose es idempotente
(`goose_db_version`) pero no serializa esas ejecuciones concurrentes con un
lock. Sin efecto mientras el despliegue sea de una sola instancia (el caso
actual).

## Fuera de alcance de este ADR

- No se versiona `render.yaml` ni configuración explícita de Vercel como IaC
  todavía — ambas plataformas se configuran manualmente vía su dashboard. Si
  se decide versionar esa configuración más adelante, es un cambio aparte.
- No hay workflow de CI que despliegue automáticamente (los 4 workflows de
  `.github/workflows/` son de calidad — lint/test/build —, ninguno hace
  deploy). Render/Vercel despliegan por su propia integración con GitHub
  (push a `main`), fuera del control de Actions.

## Referencias

- [`backend/README.md`](../../backend/README.md#despliegue-render) — env vars
  y detalle del arranque en Render.
- `backend/internal/common/migrate.go` — migraciones embebidas al arrancar.
- [ROADMAP.md](../roadmap/ROADMAP.md#infraestructura-de-despliegue) — Opción 1
  original que este ADR cierra.
