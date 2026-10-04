-- When the workout started and finished. Both are known or both are NULL.
ALTER TABLE workouts
    ADD COLUMN started_at  timestamptz,
    ADD COLUMN finished_at timestamptz,
    ADD CONSTRAINT workouts_times_check CHECK (
        (started_at IS NULL) = (finished_at IS NULL) AND finished_at >= started_at
    );

---- create above / drop below ----

ALTER TABLE workouts
    DROP CONSTRAINT workouts_times_check,
    DROP COLUMN finished_at,
    DROP COLUMN started_at;
