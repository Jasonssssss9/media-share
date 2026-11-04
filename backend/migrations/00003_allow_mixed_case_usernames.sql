-- +goose Up
ALTER TABLE users DROP CONSTRAINT users_username_check;
ALTER TABLE users ADD CONSTRAINT users_username_check
    CHECK (username ~ '^[A-Za-z0-9_]{3,24}$');

-- +goose Down
ALTER TABLE users DROP CONSTRAINT users_username_check;
ALTER TABLE users ADD CONSTRAINT users_username_check
    CHECK (username ~ '^[a-z0-9_]{3,24}$');
