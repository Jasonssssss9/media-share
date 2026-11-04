-- +goose Up
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(254) NOT NULL CHECK (btrim(email) <> ''),
    username VARCHAR(24) NOT NULL UNIQUE
        CHECK (username ~ '^[a-z0-9_]{3,24}$'),
    password_hash TEXT NOT NULL CHECK (password_hash <> ''),
    display_name VARCHAR(60) NOT NULL CHECK (btrim(display_name) <> ''),
    bio VARCHAR(280) NOT NULL DEFAULT '',
    avatar_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX users_email_key ON users (lower(email));

CREATE TABLE posts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content VARCHAR(2000) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX posts_feed_idx ON posts (created_at DESC, id DESC);
CREATE INDEX posts_author_feed_idx ON posts (author_id, created_at DESC, id DESC);

CREATE TABLE media (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    post_id UUID REFERENCES posts(id) ON DELETE SET NULL,
    storage_key TEXT NOT NULL UNIQUE CHECK (storage_key <> ''),
    mime_type TEXT NOT NULL
        CHECK (mime_type IN ('image/jpeg', 'image/png', 'image/webp')),
    size BIGINT NOT NULL CHECK (size > 0 AND size <= 5242880),
    width INTEGER NOT NULL CHECK (width > 0),
    height INTEGER NOT NULL CHECK (height > 0),
    position SMALLINT NOT NULL DEFAULT 0 CHECK (position BETWEEN 0 AND 3),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT media_post_position_key UNIQUE (post_id, position)
        DEFERRABLE INITIALLY IMMEDIATE
);

CREATE INDEX media_owner_idx ON media (owner_id);

ALTER TABLE users ADD CONSTRAINT users_avatar_id_fkey
    FOREIGN KEY (avatar_id) REFERENCES media(id) ON DELETE SET NULL;

CREATE TABLE likes (
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (post_id, user_id)
);

CREATE INDEX likes_user_idx ON likes (user_id);

CREATE TABLE comments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    post_id UUID NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
    author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    content VARCHAR(500) NOT NULL CHECK (btrim(content) <> ''),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX comments_post_idx ON comments (post_id, created_at, id);
CREATE INDEX comments_author_idx ON comments (author_id);

-- +goose Down
DROP TABLE comments;
DROP TABLE likes;
ALTER TABLE users DROP CONSTRAINT users_avatar_id_fkey;
DROP TABLE media;
DROP TABLE posts;
DROP TABLE users;
