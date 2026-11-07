# Security controls

Security is the top priority. Every phase must keep every box below that applies to it.

## In transit
- [ ] TLS 1.2+ everywhere (1.3 preferred); HSTS on every HTTPS endpoint
- [ ] Phone → Pi: Tailscale (WireGuard) + mTLS client cert + HMAC-signed body
- [ ] Pi → AWS: IoT Core mTLS with a per-device X.509 cert; IoT policy allows publishing to one topic only
- [ ] No router port forwarding; the Pi's single port (8443, mTLS) is bound to the Tailscale IP only
- [ ] S3 bucket policies deny non-TLS requests

## At rest
- [ ] Pi: LUKS full-disk encryption; secrets files `chmod 600`
- [ ] AWS: encryption at rest everywhere with AWS-owned/AWS-managed keys (free); revisit a
      customer-managed key if a kill switch or cross-account backups become necessary
- [ ] S3: Block Public Access, versioning, Object Lock on raw data
- [ ] Never store full card numbers (last 4 only), bank passwords or Wallet credentials

## Identity and access
- [ ] Root account: MFA, no access keys, unused day to day
- [ ] Humans via IAM Identity Center; CI via GitHub OIDC; no long-lived access keys anywhere
- [ ] One least-privilege IAM role per Lambda
- [ ] Cognito: self-signup disabled, MFA required (the dashboard is the only human interface)
- [ ] `cloud-api` writes: server-side validation, manual entries only, every write logged
- [ ] Android: keys in Android Keystore (non-exportable), `allowBackup=false`

## Containers and supply chain
- [ ] Distroless/minimal images, non-root, read-only root filesystem, all capabilities dropped
- [ ] Parser container has no network access
- [ ] `govulncheck`, `npm audit`, Dependabot; Trivy image scanning (phase 6)
- [ ] GitHub Actions pinned to commit SHAs; base images pinned by digest

## Detection and cost
- [ ] CloudTrail with log file validation; GuardDuty; IAM Access Analyzer
- [ ] AWS Budgets alarms ($5, $10) + Cost Anomaly Detection (cost spikes often mean compromise)
- [ ] API Gateway throttling + Lambda reserved concurrency as a hard cost ceiling
- [ ] Phone heartbeat: alert when no events arrive for N days
