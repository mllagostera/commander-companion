-- +goose Up
-- +goose StatementBegin

-- Friend requests between two app users (Stage 9 of the roadmap, "Social").
-- Deliberately distinct from playgroups (game groups): a playgroup member add
-- takes effect immediately, a friend request needs the addressee's accept.
-- No separate "friends" table: an accepted row IS the friendship (query both
-- directions with requester_id/addressee_id), same "the row's status models
-- the whole lifecycle" pattern as tournament_participants/moxfield_import_jobs.
CREATE TABLE friend_requests (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  requester_id   uuid NOT NULL REFERENCES users(id),
  addressee_id   uuid NOT NULL REFERENCES users(id),
  status         varchar NOT NULL DEFAULT 'pending',
  created_at     timestamp DEFAULT (now()),
  responded_at   timestamp,
  CONSTRAINT friend_requests_no_self_request CHECK (requester_id <> addressee_id),
  CONSTRAINT friend_requests_status_chk
    CHECK (status IN ('pending', 'accepted', 'rejected', 'cancelled'))
);

-- Only one pending request per direction at a time. The reverse direction
-- (B already has a pending request to A when A requests B) is handled by
-- internal/friends' service, which auto-accepts the existing row instead of
-- inserting a second one -- same check-then-act pattern already used by
-- playgroups.AddMember, no DB-level cross-direction constraint needed for it.
CREATE UNIQUE INDEX friend_requests_pending_direction_idx
  ON friend_requests (requester_id, addressee_id) WHERE status = 'pending';

CREATE INDEX friend_requests_addressee_id_idx ON friend_requests (addressee_id);
CREATE INDEX friend_requests_requester_id_idx ON friend_requests (requester_id);

-- Deny-all RLS on every new table, same as the rest of the public schema
-- (Supabase exposes it via PostgREST with an anon key; the backend accesses it
-- through a direct Postgres connection instead, see 00014's identical rationale).
ALTER TABLE friend_requests ENABLE ROW LEVEL SECURITY;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE friend_requests DISABLE ROW LEVEL SECURITY;

DROP TABLE friend_requests;

-- +goose StatementEnd
