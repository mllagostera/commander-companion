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

## Notas

- Gran parte del código sigue evolucionando activamente — antes de asumir que
  algo está terminado, revisá [`docs/roadmap/TASKS.md`](../docs/roadmap/TASKS.md).
