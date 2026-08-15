# ER Diagram — Commander Companion

Entity-relationship diagram generated from the real schema
(`docs/database/schema.dbml`, compiled to SQL in
`backend/migrations/00001_initial_schema.sql` through
`00012_game_player_proxy_join.sql`, see the directory for the full
listing). Keep in sync with the DBML whenever the schema changes.

```mermaid
erDiagram
    users ||--o{ decks : "owns"
    users ||--o{ refresh_tokens : "has"
    users ||--o{ playgroup_members : "is a member via"
    playgroups ||--o{ playgroup_members : "has"
    playgroups |o--o{ games : "groups (optional)"
    users ||--o{ game_players : "plays as"
    users |o--o{ game_players : "joined as proxy (added_by, optional)"
    decks ||--o{ game_players : "is used in"
    games ||--o{ game_players : "has"
    games ||--o{ game_actions : "records"
    game_players ||--o{ game_actions : "actor of"
    game_players |o--o{ game_actions : "target of (optional)"
    users ||--o| user_statistics_summary : "summary of"
    decks ||--o| deck_statistics_summary : "summary of"
    users ||--o{ friend_requests : "sent (requester_id)"
    users ||--o{ friend_requests : "received (addressee_id)"

    users {
        uuid id PK
        varchar username UK
        varchar email UK
        varchar password_hash "nullable: NULL if it's a Google-only account"
        varchar google_id UK "nullable: NULL if it's an email/password account"
        timestamp created_at
        timestamp updated_at
    }

    refresh_tokens {
        uuid id PK
        uuid user_id FK
        varchar token_hash UK "SHA-256 of the refresh token"
        timestamp expires_at
        timestamp created_at
        timestamp revoked_at "nullable: set on logout/rotation"
    }

    decks {
        uuid id PK
        uuid user_id FK
        varchar name
        varchar commander
        varchar moxfield_id "indexed (00003): import lookup/de-dup"
        varchar image_url "nullable (00006): commander art crop from Moxfield"
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
        uuid game_id FK "indexed (00003): hot path of game state"
        uuid user_id FK
        uuid deck_id FK
        int life_total "default 40"
        int poison_counters "default 0"
        int energy_counters "default 0"
        int experience_counters "default 0"
        boolean is_eliminated "default false"
        uuid added_by FK "nullable (00012): who joined them if not themselves, see ADR-0013"
    }

    game_actions {
        uuid id PK
        uuid game_id FK "indexed (00003): hot path of the timeline"
        uuid actor_id FK "game_players.id: origin of the action"
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

    friend_requests {
        uuid id PK
        uuid requester_id FK "users.id"
        uuid addressee_id FK "users.id"
        varchar status "CHECK (00017): pending | accepted | rejected | cancelled"
        timestamp created_at
        timestamp responded_at "nullable: set on accept/reject/cancel"
    }
```

## Notes

- `user_statistics_summary` and `deck_statistics_summary` are pre-computed
  summary tables (optional 1:1 with `users`/`decks`): no row exists until
  the user/deck finishes their first game (see `internal/statistics`).
- `game_actions.target_id` is optional: several actions (`TurnStart`,
  `TurnEnd`, or `LifeChange`/`Elimination` on oneself) have no target, and
  the subject of the action is `actor_id` itself.
- Explicit indexes beyond the PKs (`decks.moxfield_id`,
  `game_actions.game_id`, `game_players.game_id`) and the `CHECK`
  constraints on `games.status` / `game_actions.action_type` were added in
  `backend/migrations/00003_indices.sql` and
  `backend/migrations/00004_status_constraints.sql` respectively;
  `00005_pagination_indices.sql` adds composite indexes supporting keyset
  pagination for `/decks` and `/games` — see `docs/roadmap/TASKS.md`
  Stage 2.
- `decks.image_url` (`00006_deck_image_url.sql`) is populated during
  Moxfield import/resync (`internal/moxfield/client.go`), from the
  `main.id` field of the Moxfield response (it builds
  `https://assets.moxfield.net/cards/card-{id}-art_crop.jpg`, the same art
  crop Moxfield uses as its own `og:image`) — it is not an arbitrary URL
  from the client. See `docs/api/openapi.yaml` (`Deck.image_url`).
- `friend_requests` (`00017_friend_requests.sql`, see ADR-0017) has no
  separate `friends` table: an `accepted` row IS the friendship, resolved to
  "the other user" regardless of which side was `requester_id`. This diagram
  predates the tournaments tables (`00016`) and the async job tables
  (`moxfield_import_jobs`/`deck_resync_jobs`, `00010`/`00013`) — see
  `docs/database/schema.dbml` for the always-current full schema.
