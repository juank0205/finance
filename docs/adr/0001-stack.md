# ADR 0001: Technology stack

- Status: accepted
- Date: 2026-10-23

## Decisions

| Area | Choice | Main reason |
|---|---|---|
| Edge hardware | Raspberry Pi 5 (not ESP32) | Needs containers, disk encryption, Postgres, future news/ML |
| Transaction source | Google Wallet notifications via a custom Android `NotificationListenerService` | Wallet has no read API; a custom app avoids giving third-party automation apps notification access |
| Android | Kotlin + Jetpack Compose | First-class access to notification listener, WorkManager, Room, Keystore |
| Backend | Go, stdlib `net/http` only | Small static binaries for the Pi and fast Lambda cold starts; zero framework deps = small supply-chain surface |
| Edge DB | PostgreSQL 18 container | Real SQL for admin queries; industry skill |
| Edge runtime | Docker Compose | Simple to start; k3s or Greengrass possible later |
| UIs | React + Vite + TypeScript (cloud dashboard; the edge admin UI was dropped in ADR 0002) | One frontend stack; static builds |
| AWS | Serverless only: Lambda, HTTP API GW, DynamoDB, S3, CloudFront, Cognito, IoT Core, SQS | Cheapest tier, about $2–5/month |
| Lambda packaging | Container images + Lambda Web Adapter | Same `net/http` binary runs locally, on the Pi and in Lambda |
| IaC | Terraform (AWS), Ansible (Pi) | Most common in industry |
| CI | GitHub + Actions, OIDC to AWS | No stored cloud keys |
