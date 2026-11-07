# ADR 0002: The cloud owns user data; the edge only automates ingestion

- Status: accepted
- Date: 2026-11-07
- Supersedes: the edge admin UI and `admin-api` from ADR 0001 and the original architecture

## Context

The first design made the Pi's Postgres the source of truth, with an admin UI on the Pi for manual
entries, statement uploads, categories, parser rules and a review queue. Manual entry was then built
into the cloud dashboard. Having both would mean two places that write data and a two-way sync.

## Decision

- The Pi runs automation only: `ingest-api`, `parser`, `uploader` and Postgres as an outbox. It has no
  UI and accepts connections only from the phone (mTLS, port 8443).
- Everything a person does happens in the cloud dashboard through `cloud-api`: manual entries,
  categories and rules, net worth, statement uploads and reviewing events the parser could not read.
- DynamoDB is the source of truth. Data flows Pi → AWS only.
- Categories are assigned in the cloud (`cloud-ingest`), where they are configured, not by the parser.

## Consequences

- No sync conflicts and no second admin surface to secure (no login, sessions or audit log on the Pi).
- Manual entries work from anywhere the dashboard is reachable, without Tailscale.
- The Pi's attack surface shrinks to one mTLS port.
- If AWS is down, the dashboard and manual entry are down. Phone events still queue on the Pi.
- Removed: `frontend/edge-admin`, `backend/cmd/admin-api`, port 443 on the Pi, the Pi `audit_log` table.
