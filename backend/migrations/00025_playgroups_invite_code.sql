-- +goose Up
-- +goose StatementBegin

-- The group's shareable invite code (POST /playgroups/{id}/invite, joined
-- through POST /playgroup-invites/{code}/accept). One code per group, NULL
-- when there is no active link. Plain text rather than a hash, unlike the
-- auth tokens: members have to be able to copy the link again, and it only
-- grants joining one group. UNIQUE doubles as the lookup index. See ADR-0023.
ALTER TABLE playgroups ADD COLUMN invite_code varchar UNIQUE;

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

ALTER TABLE playgroups DROP COLUMN invite_code;

-- +goose StatementEnd
