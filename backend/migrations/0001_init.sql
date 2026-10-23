-- Edge (Pi) Postgres schema. Runs once on first container start via docker-entrypoint-initdb.d.

-- Exactly what the phone sent, kept so parsers can be fixed and re-run without data loss.
CREATE TABLE raw_events (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    source      TEXT        NOT NULL,
    payload     JSONB       NOT NULL,
    status      TEXT        NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending', 'parsed', 'failed', 'ignored')),
    error       TEXT
);
CREATE INDEX raw_events_pending ON raw_events (received_at) WHERE status = 'pending';

CREATE TABLE transactions (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    raw_event_id BIGINT      REFERENCES raw_events (id),
    occurred_at  TIMESTAMPTZ NOT NULL,
    amount_minor BIGINT      NOT NULL,          -- cents; negative = money out
    currency     CHAR(3)     NOT NULL,
    merchant     TEXT        NOT NULL,
    category     TEXT,
    card_last4   CHAR(4),
    source       TEXT        NOT NULL,
    dedup_key    TEXT        NOT NULL UNIQUE,
    synced_at    TIMESTAMPTZ                    -- NULL until uploaded to AWS
);
CREATE INDEX transactions_unsynced ON transactions (occurred_at) WHERE synced_at IS NULL;

-- Every admin action, append-only.
CREATE TABLE audit_log (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    actor      TEXT        NOT NULL,
    action     TEXT        NOT NULL,
    detail     JSONB
);
