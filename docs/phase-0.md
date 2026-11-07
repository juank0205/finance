# Phase 0: AWS account and guardrails

Steps marked **(manual)** cannot be automated or must not be. Do them in order.

## 1. Tools (manual)

```powershell
winget install GoLang.Go Hashicorp.Terraform Amazon.AWSCLI
```

## 2. Organization and account (manual)

The project lives in its own member account inside an AWS Organization. The personal (management)
account only handles billing and identity, and never runs project workloads.

```
Management account (billing + IAM Identity Center)
└── Root
    └── OU: finance
        └── Account: finance-prod   ← everything in this repo
```

1. Management account: root has MFA. **Organizations → Create organization** (all features).
2. **Root → Actions → Create OU** `finance`.
3. **Add an AWS account → Create**: name `finance-prod`, a unique email (Gmail `+` aliases work), keep
   the `OrganizationAccountAccessRole` (break-glass access from the management account). Move it into
   the `finance` OU.
4. **IAM → Root access management → Enable**, then delete the root credentials of `finance-prod`. The
   member account then has no root password or keys at all.
5. **IAM Identity Center → Enable** (organization instance, home region **us-east-1**). Then:
   - *Users → Add user* (yourself), require MFA at sign-in (*Settings → Authentication → MFA: every sign-in*).
   - *Permission sets → Create → Predefined → AdministratorAccess*, session duration 4h.
   - *AWS accounts →* `finance-prod` *→ Assign users* → you + AdministratorAccess.
6. **Billing → Billing preferences**: enable *Receive Free Tier usage alerts*.

Project resources live in **eu-west-3** (Paris). A few global pieces (root sign-in events, CloudFront
certificates) have to live in us-east-1; Terraform handles those through a second provider.

## 3. CLI login (manual)

The portal URL is in **IAM Identity Center → Dashboard → AWS access portal URL**. The SSO region is
where Identity Center lives (us-east-1), which is not the region of the resources (eu-west-3).

```powershell
aws configure sso
# SSO session name: admin
# SSO start URL: https://d-xxxxxxxxxx.awsapps.com/start
# SSO region: us-east-1
# pick finance-prod + AdministratorAccess
# Default client Region: eu-west-3
# Profile name: admin-finance-prod

aws sso login --sso-session admin
$env:AWS_PROFILE = "admin-finance-prod"
aws sts get-caller-identity   # must show finance-prod and an AWSReservedSSO_AdministratorAccess role
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

## 5. Free-tier stack and dashboard CD

Continue with [deploy-dashboard.md](deploy-dashboard.md): budget alert, GitHub OIDC roles and the
dashboard on S3 + CloudFront, all inside the free tier.

The `security-baseline` module (CloudTrail, GuardDuty, root sign-in alert, account hardening) is
written but not deployed: as it stands it costs about $1-3/month (a customer-managed KMS key and
GuardDuty after its trial). It will be reworked to fit the free tier before it is enabled.
