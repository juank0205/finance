# finance

Personal finance platform and cloud-computing learning project.

Google Wallet payment notifications on an Android phone are forwarded to a Raspberry Pi, which
parses, cleans and stores them, offers a private admin UI for manual uploads, and syncs the clean
data to a low-cost serverless AWS backend that serves a dashboard.

```
Android (Kotlin) ──mTLS──▶ Raspberry Pi (Docker Compose)  ──mTLS/MQTT──▶ AWS IoT Core ─▶ SQS ─▶ Lambda ─▶ DynamoDB + S3
                           caddy · ingest-api · admin-api                                                     │
Browser (tailnet) ───────▶ parser · uploader · postgres        Browser ─▶ CloudFront ─▶ S3 dashboard           │
                           edge-admin UI (React)                        └▶ API Gateway (Cognito) ─▶ Lambda ◀──┘
```

See [docs/architecture.md](docs/architecture.md), [docs/security.md](docs/security.md) and
[docs/adr/0001-stack.md](docs/adr/0001-stack.md).

## Layout

| Path | What | Stack |
|---|---|---|
| `android/` | Wallet notification forwarder | Kotlin, Jetpack Compose |
| `backend/` | All Go services, edge and cloud (one module) | Go stdlib `net/http` |
| `backend/cmd/ingest-api` | Receives phone events (behind mTLS) | edge |
| `backend/cmd/admin-api` | API for the edge admin UI | edge |
| `backend/cmd/parser` | raw events → clean transactions (no internet) | edge |
| `backend/cmd/uploader` | Syncs transactions to AWS IoT Core | edge |
| `backend/cmd/cloud-ingest` | SQS consumer → DynamoDB/S3 | Lambda |
| `backend/cmd/cloud-api` | Dashboard API | Lambda |
| `backend/migrations` | Edge Postgres schema | PostgreSQL 18 |
| `edge/` | Pi runtime: Compose file, Caddy, secrets | Docker Compose, Caddy |
| `frontend/edge-admin` | Pi admin UI: uploads, review queue, rules | React + Vite + TS |
| `frontend/cloud-dashboard` | Cloud dashboard | React + Vite + TS |
| `infra/terraform` | AWS infrastructure | Terraform |
| `infra/ansible` | Raspberry Pi provisioning | Ansible (run from WSL) |
| `.github/` | CI + Dependabot | GitHub Actions |

## Prerequisites (dev machine)

- Go 1.27+ · Node 22+ · Docker Desktop (with buildx) · Terraform 1.10+ · Android Studio
- WSL (Ubuntu) for Ansible
- AWS account with root MFA enabled and an IAM Identity Center user (no access keys)

## Roadmap

- [ ] **0** AWS account hardening, Terraform bootstrap (state bucket), `security-baseline` module, GitHub repo + OIDC
- [ ] **1** Pi: OS with LUKS, Ansible roles (base, tailscale, docker)
- [ ] **2** Android listener app → `ingest-api` → `parser` → Postgres
- [ ] **3** Edge admin UI + `admin-api` (login, manual entry, CSV/PDF upload, review queue)
- [ ] **4** `uploader` → IoT Core → SQS → `cloud-ingest` → DynamoDB/S3 (`data`, `iot` modules)
- [ ] **5** Cloud dashboard + `cloud-api` (`api`, `web` modules)
- [ ] **6** CD pipeline: image builds, scanning, signed deploys
- [ ] **7** News fetching + Bedrock summaries for investment research

## Common commands

```bash
# Frontends
cd frontend/edge-admin && npm install && npm run dev

# Go services
cd backend && go run ./cmd/admin-api

# Build an edge image for the Pi (arm64)
cd backend && docker buildx build --platform linux/arm64 --target edge --build-arg CMD=ingest-api -t finance/ingest-api .

# Terraform bootstrap (once)
cd infra/terraform/bootstrap && terraform init && terraform apply
```
