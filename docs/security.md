# Security controls

Security is the top priority. Every phase must keep every box below that applies to it.

## In transit
- [ ] TLS 1.2+ everywhere (1.3 preferred); HSTS on every HTTPS endpoint
- [ ] Phone → Pi: Tailscale (WireGuard) + mTLS client cert + HMAC-signed body
- [ ] Pi → AWS: IoT Core mTLS with a per-device X.509 cert; IoT policy allows publishing to one topic only
- [ ] No router port forwarding; edge ports bound to the Tailscale IP only
- [ ] S3 bucket policies deny non-TLS requests

## At rest
- [ ] Pi: LUKS full-disk encryption; secrets files `chmod 600`
- [ ] AWS: customer-managed KMS key (rotation on) for DynamoDB, S3, SQS, CloudWatch Logs
- [ ] S3: Block Public Access, versioning, Object Lock on raw data
- [ ] Never store full card numbers (last 4 only), bank passwords or Wallet credentials

## Identity and access
- [ ] Root account: MFA, no access keys, unused day to day
- [ ] Humans via IAM Identity Center; CI via GitHub OIDC; no long-lived access keys anywhere
- [ ] One least-privilege IAM role per Lambda
- [ ] Cognito: self-signup disabled, MFA required
- [ ] Edge admin: password + TOTP/passkey, audit log of every write
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
