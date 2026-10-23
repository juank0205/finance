# Phase 0: AWS account and guardrails

Steps marked **(manual)** cannot be automated or must not be. Do them in order.

## 1. Tools (manual)

```powershell
winget install GoLang.Go Hashicorp.Terraform Amazon.AWSCLI
```

## 2. Account (manual)

1. Create the AWS account at <https://aws.amazon.com>. Use an email only for this, with its own strong password.
2. Root user: **Security credentials → Assign MFA** (passkey or authenticator app). Never create root access keys.
3. **Billing → Billing preferences**: enable *Receive Free Tier usage alerts*.
4. Choose region **us-east-1** (top right) and keep it everywhere.
5. **IAM Identity Center → Enable** (organization instance). Then:
   - *Users → Add user* (yourself), require MFA at sign-in (*Settings → Authentication → MFA: every sign-in*).
   - *Permission sets → Create → Predefined → AdministratorAccess*, session duration 4h.
   - *AWS accounts →* select the account *→ Assign users* → you + AdministratorAccess.
6. Sign out of root. From now on use only the Identity Center portal / CLI.

## 3. CLI login (manual)

```powershell
aws configure sso          # session name: finance, region us-east-1, profile name: finance
aws sso login --profile finance
$env:AWS_PROFILE = "finance"
aws sts get-caller-identity   # must show an AWSReservedSSO_AdministratorAccess role, not root
```

## 4. Terraform state bucket (once)

```powershell
cd infra/terraform/bootstrap
terraform init
terraform apply
terraform output state_bucket    # e.g. finance-tfstate-123456789012
```

Back up `bootstrap/terraform.tfstate` somewhere private (it is git-ignored). It only describes the
state bucket, so losing it is an inconvenience, not a disaster.

## 5. Security baseline

1. In `infra/terraform/envs/prod/versions.tf` replace `ACCOUNT_ID` with your 12-digit account ID.
2. Copy `terraform.tfvars.example` to `terraform.tfvars` and set your alert email.
3. Check **Billing → Cost Anomaly Detection**. If there is no monitor yet, add
   `create_cost_anomaly_monitor = true` to the module block; if a default one exists, just add your
   email to its alert subscription in the console.
4. Apply:

```powershell
cd infra/terraform/envs/prod
terraform init
terraform plan      # read it: everything should be "create", nothing destroyed
terraform apply
```

5. **(manual)** Confirm the SNS subscription email ("AWS Notification - Subscription Confirmation").

## 6. CI plan role

GitHub repo → *Settings → Secrets and variables → Actions → Variables*:

| Variable | Value |
|---|---|
| `AWS_PLAN_ROLE_ARN` | `terraform output github_plan_role_arn` |
| `ALERT_EMAIL` | your alert email |

The next push runs `terraform plan` in CI with short-lived, read-only credentials.

## Result

| Control | Cost |
|---|---|
| CloudTrail (all regions, log validation, KMS) | ~$0 (first trail free; S3 cents) |
| GuardDuty → email for medium+ findings | ~$0–2 (30-day free trial) |
| Root sign-in → email | $0 |
| Budget alarms at $5, $10 (actual) and $10 (forecast) | $0 |
| IAM Access Analyzer, S3 account Block Public Access, EBS default encryption, password policy | $0 |
| Customer-managed KMS key (reused by later phases) | $1 |
| GitHub OIDC read-only plan role | $0 |
