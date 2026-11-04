-- +goose Up
ALTER TABLE users ADD COLUMN session_version INTEGER NOT NULL DEFAULT 0
    CHECK (session_version >= 0);

-- +goose Down
ALTER TABLE users DROP COLUMN session_version;
