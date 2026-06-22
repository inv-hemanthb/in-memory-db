CREATE TABLE items (
    id          BIGSERIAL PRIMARY KEY,
    key         VARCHAR(255) NOT NULL,
    value       TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at  TIMESTAMPTZ
);

CREATE UNIQUE INDEX items_key_live_uq ON items (key) WHERE deleted_at IS NULL;
