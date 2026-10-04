-- Web sign-in. Tokens are stored only as SHA-256 hashes.

-- A pending browser sign-in: token_hash travels in the bot deep link, nonce_hash
-- in the browser's login cookie. An identity provider sets user_id and
-- confirmed_at; the browser's poll sets consumed_at when it takes the session.
CREATE TABLE login_requests (
    id           bigserial   PRIMARY KEY,
    token_hash   bytea       NOT NULL UNIQUE,
    nonce_hash   bytea       NOT NULL UNIQUE,
    user_id      bigint      REFERENCES users (id) ON DELETE CASCADE,
    created_at   timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL,
    confirmed_at timestamptz,
    consumed_at  timestamptz
);

CREATE TABLE sessions (
    id         bigserial   PRIMARY KEY,
    token_hash bytea       NOT NULL UNIQUE,
    user_id    bigint      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL
);

CREATE INDEX sessions_user_id_idx ON sessions (user_id);

---- create above / drop below ----

DROP TABLE sessions;
DROP TABLE login_requests;
