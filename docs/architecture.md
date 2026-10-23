# Architecture

## Data flow

1. **Phone.** A Google Wallet payment posts a notification. The Android app's `NotificationListenerService`
   captures it, queues it in Room, signs it (HMAC) and POSTs it to the Pi over Tailscale with a
   client certificate.
2. **Pi, ingest.** Caddy verifies the client cert (port 8443) and forwards to `ingest-api`, which
   checks the HMAC and stores the payload untouched in `raw_events`.
3. **Pi, parse.** `parser` (no network access) turns pending raw events into `transactions`: amount in
   minor units, currency, merchant, card last 4, category, and a `dedup_key`. Failures go to the
   review queue in the admin UI.
4. **Pi, admin.** The browser reaches the admin UI on port 443 over the tailnet. Manual entries and
   statement uploads are parsed by the same `parser`.
5. **Pi to AWS.** `uploader` publishes transactions where `synced_at IS NULL` to IoT Core (MQTT, mTLS
   with the device X.509 cert) and marks them synced on ack.
6. **AWS, ingest.** An IoT rule sends messages to SQS. `cloud-ingest` (Lambda) upserts by `dedup_key`
   into DynamoDB, updates monthly aggregates and archives raw JSON to S3. Poison messages land in a
   dead-letter queue.
7. **AWS, dashboard.** CloudFront serves the React build from a private S3 bucket. The API goes through
   API Gateway (Cognito JWT authorizer, throttling) to `cloud-api` (Lambda).

The Pi's Postgres is the local source of truth and offline buffer. AWS holds the synced copy for the
dashboard and analytics (Athena over S3).

## Why these AWS services (cheapest secure tier, about $2–5/month)

| Need | Choice | Instead of |
|---|---|---|
| Compute | Lambda (arm64 container images + Lambda Web Adapter) | Fargate + ALB |
| Database | DynamoDB on-demand (+ Athena on S3 for ad-hoc SQL) | RDS |
| Network | No VPC; IAM-authenticated TLS to AWS APIs | VPC + NAT Gateway |
| Secrets | SSM Parameter Store SecureString | Secrets Manager |
| Edge connectivity | IoT Core (per-device X.509) | Long-lived IAM keys on the Pi |

## Edge networks (edge/compose.yaml)

| Network | Members | Internet |
|---|---|---|
| `public` | caddy | published ports, bound to the Tailscale IP only |
| `front` | caddy, ingest-api, admin-api | no |
| `data` | APIs, parser, uploader, postgres | no |
| `egress` | uploader | yes (IoT Core) |
