-- Normalized key used for exercise names and aliases:
-- trim, collapse internal whitespace, lowercase.
-- NOTE: lower() on Cyrillic requires a UTF-8 locale or ICU collation on the
-- database; with LC_CTYPE=C it only folds ASCII.

CREATE TABLE users (
    id         bigserial   PRIMARY KEY,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE user_identities (
    id          bigserial   PRIMARY KEY,
    user_id     bigint      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider    text        NOT NULL,
    external_id text        NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (provider, external_id)
);

CREATE INDEX user_identities_user_id_idx ON user_identities (user_id);

CREATE TABLE exercises (
    id         bigserial   PRIMARY KEY,
    user_id    bigint      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    name       text        NOT NULL CHECK (btrim(name) <> ''),
    name_key   text        NOT NULL GENERATED ALWAYS AS (lower(regexp_replace(btrim(name), '\s+', ' ', 'g'))) STORED,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, name_key)
);

CREATE TABLE exercise_aliases (
    id          bigserial   PRIMARY KEY,
    user_id     bigint      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    exercise_id bigint      NOT NULL REFERENCES exercises (id) ON DELETE CASCADE,
    alias       text        NOT NULL CHECK (btrim(alias) <> ''),
    alias_key   text        NOT NULL GENERATED ALWAYS AS (lower(regexp_replace(btrim(alias), '\s+', ' ', 'g'))) STORED,
    created_at  timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, alias_key)
);

CREATE INDEX exercise_aliases_exercise_id_idx ON exercise_aliases (exercise_id);

CREATE TABLE workouts (
    id           bigserial   PRIMARY KEY,
    user_id      bigint      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    performed_on date        NOT NULL,
    workout_type text,
    kcal         integer     CHECK (kcal >= 0),
    protein_g    integer     CHECK (protein_g >= 0),
    note         text,
    raw_text     text        NOT NULL,
    source       text        NOT NULL,
    source_ref   text        NOT NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    UNIQUE (user_id, source, source_ref)
);

CREATE INDEX workouts_user_id_performed_on_idx ON workouts (user_id, performed_on);

CREATE TABLE workout_exercises (
    id              bigserial PRIMARY KEY,
    workout_id      bigint    NOT NULL REFERENCES workouts (id) ON DELETE CASCADE,
    exercise_id     bigint    NOT NULL REFERENCES exercises (id) ON DELETE RESTRICT,
    position        integer   NOT NULL CHECK (position > 0),
    name_as_written text      NOT NULL,
    UNIQUE (workout_id, position)
);

CREATE INDEX workout_exercises_exercise_id_idx ON workout_exercises (exercise_id);

CREATE TABLE exercise_sets (
    id                  bigserial    PRIMARY KEY,
    workout_exercise_id bigint       NOT NULL REFERENCES workout_exercises (id) ON DELETE CASCADE,
    position            integer      NOT NULL CHECK (position > 0),
    weight_kg           numeric(6,2) NOT NULL DEFAULT 0 CHECK (weight_kg >= 0),
    reps                integer      NOT NULL CHECK (reps > 0),
    UNIQUE (workout_exercise_id, position)
);

---- create above / drop below ----

DROP TABLE exercise_sets;
DROP TABLE workout_exercises;
DROP TABLE workouts;
DROP TABLE exercise_aliases;
DROP TABLE exercises;
DROP TABLE user_identities;
DROP TABLE users;
