# Commander Companion — Backend

API en Go (Fiber + PostgreSQL + sqlc + goose). Contrato REST único para los
clientes Android y web, ver [`docs/api/openapi.yaml`](../docs/api/openapi.yaml)
(fuente de verdad — si cambia el comportamiento de un endpoint, ese archivo
se actualiza en el mismo cambio).

## Stack

Go + Fiber, PostgreSQL, `sqlc` (acceso a datos tipado), `goose` (migraciones).

## Setup

```bash
cd backend
cp .env.example .env   # completar DB_URL, JWT_SECRET, etc.
make migrate-up        # aplicar migraciones contra tu Postgres
make run                # http://localhost:8080
```

Variables de entorno documentadas en [`.env.example`](.env.example).

## Comandos

```bash
make run                 # levantar la API localmente
make test                # go test -race ./... (algunos son de integración, requieren Postgres)
make lint                # golangci-lint (make lint-docker si no lo tenés instalado)
make generate-sql        # regenerar repositorios con sqlc tras editar query.sql
make migrate-up          # aplicar migraciones goose
make migrate-down        # revertir la última migración
```

## Probar todo el stack con Docker

Para levantar la API junto con Postgres (y opcionalmente el cliente web), ver
el `docker-compose.yml` en la raíz del repo:

```bash
cd ..
docker compose up --build
```

La primera vez hay que aplicar las migraciones (no corren solas dentro del
contenedor): `make migrate-up` con `DB_URL` apuntando a `localhost:5432`
(el puerto que publica el servicio `db` de Compose).

## Despliegue (Render)

El propio binario `api` aplica las migraciones de goose al arrancar, antes de
levantar el servidor HTTP (ver `common.RunMigrations` en
[`cmd/api/main.go`](cmd/api/main.go)). No depende de un "Pre-Deploy Command" ni
de ningún otro hook de la plataforma — corre igual en Render (incluido el
free tier, que no ofrece ese hook), en Docker Compose o localmente. La imagen
de [`Dockerfile`](Dockerfile) incluye el directorio `migrations/` junto al
binario para que esto funcione en runtime.

Nota de escala: si el servicio corre con más de una réplica, todas las
instancias ejecutan `goose up` al bootear en paralelo; goose es idempotente
(cada migración se aplica una sola vez, trackeado en `goose_db_version`) pero
no serializa esas ejecuciones concurrentes con un lock. Con una sola réplica
(el caso actual) no aplica.

Variables de entorno mínimas a configurar en el servicio de Render: `DB_URL`
(connection string de Supabase — usar el **Session pooler**, no el
Transaction pooler, porque este backend usa prepared statements vía pgx),
`JWT_SECRET` (uno nuevo, no el default de dev), `GOOGLE_CLIENT_ID`,
`CORS_ALLOWED_ORIGINS` y `WEB_APP_URL` (dominio del frontend en Vercel). Ver
[`.env.example`](.env.example) para el resto.

## Notas

- Gran parte del código sigue evolucionando activamente — antes de asumir que
  algo está terminado, revisá [`docs/roadmap/TASKS.md`](../docs/roadmap/TASKS.md).
