# Arquitectura y Principios de Diseño

Este documento describe la arquitectura y los principios fundamentales de diseño del proyecto **Commander Companion**.

## Las 4 Fuentes de Verdad

Para garantizar que el proyecto pueda escalar y que el desarrollo paralelo (incluyendo la colaboración con IAs) sea eficiente y coherente, el sistema se basa en cuatro pilares documentales:

1. **DBML (Database Markup Language):**
   Define el esquema de la base de datos, tipos, relaciones, índices y restricciones.
   Ubicación: `docs/database/schema.dbml`

2. **OpenAPI 3.1:**
   Es el contrato único e inviolable entre el backend (Go) y los clientes (Android). Cualquier cambio en la comunicación debe reflejarse primero aquí.
   Ubicación: `docs/api/openapi.yaml`

3. **Mermaid (Diagramas):**
   Utilizados para documentar arquitectura, flujos de datos, máquinas de estado y comportamiento de los sistemas.
   Ubicación: Dentro de los Markdowns en `docs/architecture/` y `docs/diagrams/`.

4. **ADR (Architecture Decision Records):**
   Documentación estructurada sobre decisiones técnicas clave, contexto, opciones consideradas y consecuencias.
   Ubicación: `docs/decisions/`

---

## Arquitectura del Sistema

### Backend (Go)
- **Patrón:** Monolito Modular.
- **Estructura Interna:** Clean Architecture enfocada en casos de uso (Service).
  - `Handler`: Capa de transporte (HTTP/REST, Websocket).
  - `Service`: Lógica de negocio (pura, sin dependencias de infraestructura).
  - `Repository`: Persistencia y acceso a datos.

### Cliente (Android)
- **Patrón:** Clean Architecture + MVVM + UDF (Unidirectional Data Flow).
- **Módulos Lógicos:**
  - `Presentation`: UI con Jetpack Compose y ViewModels.
  - `Domain`: Casos de uso e interfaces de repositorios.
  - `Data`: Implementación de repositorios (Retrofit, Room).
