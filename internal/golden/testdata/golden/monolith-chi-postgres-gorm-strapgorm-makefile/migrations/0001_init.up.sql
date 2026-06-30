-- region: migrasi awal (skeleton). Mode: copy (disalin apa adanya, tanpa render).
CREATE TABLE IF NOT EXISTS health_check (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    checked_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
