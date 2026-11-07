# finance

Personal finance platform and cloud-computing learning project.

Google Wallet payment notifications on an Android phone are forwarded to a Raspberry Pi, which
parses them and syncs clean transactions to a low-cost serverless AWS backend. The Pi only automates;
the cloud dashboard is where you view everything, add manual entries and manage categories.

```
Android (Kotlin) ──mTLS──▶ Raspberry Pi (Docker Compose)  ──mTLS/MQTT──▶ AWS IoT Core ─▶ SQS ─▶ Lambda ─▶ DynamoDB + S3
                           caddy · ingest-api · parser                                                        │
                           uploader · postgres (outbox)        Browser ─▶ CloudFront ─▶ S3 dashboard           │
                                                                        └▶ API Gateway (Cognito) ─▶ Lambda ◀──┘
```

See [docs/architecture.md](docs/architecture.md), [docs/security.md](docs/security.md),
[docs/adr/0001-stack.md](docs/adr/0001-stack.md) and
[docs/adr/0002-cloud-owns-user-data.md](docs/adr/0002-cloud-owns-user-data.md).

## Layout

| Path | What | Stack |
|---|---|---|
| `android/` | Wallet notification forwarder | Kotlin, Jetpack Compose |
| `backend/` | All Go services, edge and cloud (one module) | Go stdlib `net/http` |
| `backend/cmd/ingest-api` | Receives phone events (behind mTLS) | edge |
| `backend/cmd/parser` | raw events → clean transactions (no internet) | edge |
| `backend/cmd/uploader` | Syncs transactions to AWS IoT Core | edge |
| `backend/cmd/cloud-ingest` | SQS consumer → DynamoDB/S3 | Lambda |
| `backend/cmd/cloud-api` | Dashboard API, including manual entries and categories | Lambda |
| `backend/migrations` | Edge Postgres schema (raw events + outbox) | PostgreSQL 18 |
| `edge/` | Pi runtime: Compose file, Caddy, secrets | Docker Compose, Caddy |
| `frontend/cloud-dashboard` | Dashboard: charts, net worth, manual entries, categories | React + Vite + TS |
| `infra/terraform` | AWS infrastructure | Terraform |
| `infra/ansible` | Raspberry Pi provisioning | Ansible (run from WSL) |
| `.github/` | CI + Dependabot | GitHub Actions |

## Prerequisites (dev machine)

- Go 1.27+ · Node 22+ · Docker Desktop (with buildx) · Terraform 1.10+ · Android Studio
- WSL (Ubuntu) for Ansible
- AWS account with root MFA enabled and an IAM Identity Center user (no access keys)

## Roadmap

- [ ] **0** AWS account hardening, Terraform bootstrap, `security-baseline` module, CI plan role ([guide](docs/phase-0.md))
- [ ] **1** Pi: OS with LUKS, Ansible roles (base, tailscale, docker)
- [ ] **2** Android listener app → `ingest-api` → `parser` → Postgres
- [ ] **3** ~~Edge admin UI~~ dropped: everything a person does moved to the cloud ([ADR 0002](docs/adr/0002-cloud-owns-user-data.md))
- [ ] **4** `uploader` → IoT Core → SQS → `cloud-ingest` → DynamoDB/S3 (`data`, `iot` modules)
- [ ] **5** Cloud dashboard + `cloud-api` (`api`, `web` modules): UI done on demo data; hosting + CD ready ([guide](docs/deploy-dashboard.md)); next: Cognito, then the real API
- [ ] **5b** In the dashboard: categorization rules, statement upload, review of failed parses
- [ ] **6** CD pipeline: image builds, scanning, signed deploys
- [ ] **7** News fetching + Bedrock summaries for investment research

## Common commands

```bash
# Dashboard (demo data unless VITE_API_URL is set)
cd frontend/cloud-dashboard && npm install && npm run dev

# Go services
cd backend && go run ./cmd/cloud-api

# Build an edge image for the Pi (arm64)
cd backend && docker buildx build --platform linux/arm64 --target edge --build-arg CMD=ingest-api -t finance/ingest-api .

# Terraform bootstrap (once)
cd infra/terraform/bootstrap && terraform init && terraform apply
```
