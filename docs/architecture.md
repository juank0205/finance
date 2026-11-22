# Architecture

The Pi automates ingestion and nothing else. Everything a person does, from viewing to manual entries,
categories, net worth and fixing failed parses, happens in the cloud dashboard. See
[ADR 0002](adr/0002-cloud-owns-user-data.md).

## Data flow

1. **Phone.** A Google Wallet payment posts a notification. The Android app's `NotificationListenerService`
   captures it, queues it in Room, signs it (HMAC) and POSTs it to the Pi over Tailscale with a
   client certificate.
2. **Pi, ingest.** Caddy verifies the client cert (port 8443) and forwards to `ingest-api`, which
   checks the HMAC and stores the payload untouched in `raw_events`.
3. **Pi, parse.** `parser` (no network access) turns pending raw events into `transactions`: kind,
   amount in minor units, currency, merchant, card last 4 and a `dedup_key`. It does not categorize.
   Events it cannot parse are marked `failed`.
4. **Pi to AWS.** `uploader` publishes unsynced transactions, and failed raw events, to IoT Core (MQTT,
   mTLS with the device X.509 cert) and marks them synced on ack.
5. **AWS, ingest.** An IoT rule sends messages to SQS. `cloud-ingest` (Lambda) assigns a category from
   the user's rules, upserts by `dedup_key` into DynamoDB, updates monthly aggregates and archives raw
   JSON to S3. Failed raw events become review items. Poison messages land in a dead-letter queue.
6. **AWS, dashboard.** CloudFront serves the React build from a private S3 bucket. The browser signs
   in with Cognito (hosted page, PKCE) and calls `cloud-api` (Lambda) through its Function URL;
   `cloud-api` checks the Cognito access token on every request. Until AWS enables CloudFront for the
   account, the dashboard runs locally against the same API ([deploy-api.md](deploy-api.md)). The API
   covers reading summaries and transactions, manual entries (`POST`/`PUT`/`DELETE /v1/transactions`), recurring rules
   (`/v1/recurring`), categories and net worth.
7. **AWS, recurring.** `cloud-api` creates the occurrences of recurring rules (mortgage, phone bill,
   salary) that are due, as normal transactions with source `recurring`, on the first request of each
   day, so no scheduled job is needed. Each rule remembers the last day it covered and only creates
   later days: editing a rule changes only the future, and a deleted occurrence never comes back. The
   dedup key `recurring:<rule>:<date>` makes a repeated run harmless. Rules are for direct debits and
   transfers; card payments already arrive from the phone.

**Source of truth:** DynamoDB in AWS. The Pi's Postgres is an outbox and raw-event archive: it buffers
while AWS is unreachable and keeps the original notification text so a fixed parser can re-run. Data
only flows Pi → AWS, never back, so there are no sync conflicts.

## Why these AWS services (cheapest secure tier, aiming for $0 inside the free tier)

| Need | Choice | Instead of |
|---|---|---|
| Compute | Lambda (arm64 container images + Lambda Web Adapter) | Fargate + ALB |
| Database | DynamoDB on-demand (+ Athena on S3 for ad-hoc SQL) | RDS |
| Network | No VPC; IAM-authenticated TLS to AWS APIs | VPC + NAT Gateway |
| Secrets | SSM Parameter Store SecureString | Secrets Manager |
| Edge connectivity | IoT Core (per-device X.509) | Long-lived IAM keys on the Pi |
| Encryption at rest | AWS-owned/AWS-managed keys | Customer-managed KMS keys ($1/month each) |

## Edge networks (edge/compose.yaml)

| Network | Members | Internet |
|---|---|---|
| `public` | caddy | published port 8443 only, bound to the Tailscale IP |
| `front` | caddy, ingest-api | no |
| `data` | ingest-api, parser, uploader, postgres | no |
| `egress` | uploader | yes (IoT Core) |
