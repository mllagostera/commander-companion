# Diagrama ER — Commander Companion

Diagrama entidad-relación generado a partir del esquema real (`docs/database/schema.dbml`,
compilado a SQL en `backend/migrations/00001_initial_schema.sql` +
`00002_auth.sql` + `00003_indices.sql` + `00004_status_constraints.sql` +
`00005_pagination_indices.sql` + `00006_deck_image_url.sql`).
Mantener sincronizado con el DBML cada vez que cambie el esquema.

```mermaid
erDiagram
    users ||--o{ decks : "posee"
    users ||--o{ refresh_tokens : "tiene"
    users ||--o{ playgroup_members : "es miembro via"
    playgroups ||--o{ playgroup_members : "tiene"
    playgroups |o--o{ games : "agrupa (opcional)"
    users ||--o{ game_players : "juega como"
    decks ||--o{ game_players : "se usa en"
    games ||--o{ game_players : "tiene"
    games ||--o{ game_actions : "registra"
    game_players ||--o{ game_actions : "actor de"
    game_players |o--o{ game_actions : "target de (opcional)"
    users ||--o| user_statistics_summary : "resumen de"
    decks ||--o| deck_statistics_summary : "resumen de"

    users {
        uuid id PK
        varchar username UK
        varchar email UK
        varchar password_hash "nullable: NULL si es cuenta solo-Google"
        varchar google_id UK "nullable: NULL si es cuenta email/password"
        timestamp created_at
        timestamp updated_at
    }

    refresh_tokens {
        uuid id PK
        uuid user_id FK
        varchar token_hash UK "SHA-256 del refresh token"
        timestamp expires_at
        timestamp created_at
        timestamp revoked_at "nullable: se setea en logout/rotación"
    }

    decks {
        uuid id PK
        uuid user_id FK
        varchar name
        varchar commander
        varchar moxfield_id "indexed (00003): lookup/de-dup de imports"
        varchar image_url "nullable (00006): art crop del comandante desde Moxfield"
        timestamp created_at
        timestamp updated_at
    }

    playgroups {
        uuid id PK
        varchar name
        timestamp created_at
        timestamp updated_at
    }

    playgroup_members {
        uuid playgroup_id PK, FK
        uuid user_id PK, FK
        timestamp joined_at
    }

    games {
        uuid id PK
        uuid playgroup_id FK "nullable"
        varchar status "CHECK (00004): pending | active | finished"
        timestamp started_at
        timestamp finished_at
        timestamp created_at
    }

    game_players {
        uuid id PK
        uuid game_id FK "indexed (00003): hot path del estado de la partida"
        uuid user_id FK
        uuid deck_id FK
        int life_total "default 40"
        int poison_counters "default 0"
        int energy_counters "default 0"
        int experience_counters "default 0"
        boolean is_eliminated "default false"
    }

    game_actions {
        uuid id PK
        uuid game_id FK "indexed (00003): hot path del timeline"
        uuid actor_id FK "game_players.id: origen de la acción"
        uuid target_id FK "nullable, game_players.id"
        varchar action_type "CHECK (00004): LifeChange | CombatDamage | CommanderDamage | PoisonCounter | TurnStart | TurnEnd | Elimination"
        jsonb payload
        timestamp created_at
    }

    user_statistics_summary {
        uuid user_id PK, FK
        int games_played "default 0"
        int games_won "default 0"
        int total_damage_dealt "default 0"
        int total_commander_damage_dealt "default 0"
        int total_eliminations "default 0"
        timestamp last_recalculated_at
    }

    deck_statistics_summary {
        uuid deck_id PK, FK
        int games_played "default 0"
        int games_won "default 0"
        int highest_life_total_achieved "default 0"
        int total_commander_damage_dealt "default 0"
        timestamp last_recalculated_at
    }
```

## Notas

- `user_statistics_summary` y `deck_statistics_summary` son tablas de resumen
  pre-calculado (1:1 opcional con `users`/`decks`): no existe fila hasta que el
  usuario/deck termina su primera partida (ver `internal/statistics`).
- `game_actions.target_id` es opcional: varias acciones (`TurnStart`,
  `TurnEnd`, o `LifeChange`/`Elimination` sobre uno mismo) no tienen target,
  y el sujeto de la acción es el propio `actor_id`.
- Índices explícitos más allá de las PK (`decks.moxfield_id`,
  `game_actions.game_id`, `game_players.game_id`) y los `CHECK` de
  `games.status` / `game_actions.action_type` se agregaron en
  `backend/migrations/00003_indices.sql` y
  `backend/migrations/00004_status_constraints.sql` respectivamente;
  `00005_pagination_indices.sql` agrega índices compuestos de apoyo al
  paginado keyset de `/decks` y `/games` — ver `docs/roadmap/TASKS.md`
  Stage 2.
- `decks.image_url` (`00006_deck_image_url.sql`) se completa en el import/
  resync de Moxfield (`internal/moxfield/client.go`), a partir del campo
  `main.id` de la respuesta de Moxfield (arma
  `https://assets.moxfield.net/cards/card-{id}-art_crop.jpg`, el mismo art
  crop que Moxfield usa como su propio `og:image`) — no es una URL
  arbitraria del cliente. Ver `docs/api/openapi.yaml` (`Deck.image_url`).
