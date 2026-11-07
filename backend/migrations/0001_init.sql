-- Edge (Pi) Postgres schema. Runs once on first container start via docker-entrypoint-initdb.d.
-- The Pi only automates ingestion: it keeps raw phone events and an outbox of parsed transactions
-- until AWS acknowledges them. Categories, notes and manual entries live in the cloud.

-- Exactly what the phone sent, kept so parsers can be fixed and re-run without data loss.
CREATE TABLE raw_events (
    id          BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    source      TEXT        NOT NULL,
    payload     JSONB       NOT NULL,
    status      TEXT        NOT NULL DEFAULT 'pending'
                CHECK (status IN ('pending', 'parsed', 'failed', 'ignored')),
    error       TEXT,
    synced_at   TIMESTAMPTZ                    -- failed events are uploaded too, for review in the dashboard
);
CREATE INDEX raw_events_pending ON raw_events (received_at) WHERE status = 'pending';

CREATE TABLE transactions (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    raw_event_id BIGINT      REFERENCES raw_events (id),
    kind         TEXT        NOT NULL DEFAULT 'expense'
                 CHECK (kind IN ('expense', 'income', 'investment')),
    occurred_at  TIMESTAMPTZ NOT NULL,
    amount_minor BIGINT      NOT NULL,          -- cents; negative = money out
    currency     CHAR(3)     NOT NULL,
    merchant     TEXT        NOT NULL,
    card_last4   CHAR(4),
    source       TEXT        NOT NULL,
    dedup_key    TEXT        NOT NULL UNIQUE,
    synced_at    TIMESTAMPTZ                    -- NULL until uploaded to AWS
);
CREATE INDEX transactions_unsynced ON transactions (occurred_at) WHERE synced_at IS NULL;
CREATE INDEX raw_events_failed_unsynced ON raw_events (received_at) WHERE status = 'failed' AND synced_at IS NULL;
