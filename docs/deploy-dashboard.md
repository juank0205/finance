# Deploying the dashboard (S3 + CloudFront, CD from GitHub Actions)

Everything here stays inside the AWS free tier. Prerequisites: phase 0 steps 1-3 (account, Identity
Center, CLI profile). Run the commands in PowerShell from the repo root, in a terminal opened after
installing Terraform.

```powershell
aws sso login --sso-session admin
$env:AWS_PROFILE = "admin-finance-prod"
aws sts get-caller-identity --query Account --output text   # your 12-digit account ID; used below
```

## 1. Push the repo to GitHub

Your GitHub CLI token needs the `workflow` scope to push files under `.github/workflows/`:

```powershell
gh auth refresh -h github.com -s workflow
git push -u origin main
```

GitHub runs `ci`. Expected: `backend`, `frontend` and `terraform` pass; `plan` is skipped. The
`deploy-dashboard` workflow is skipped too until step 5 sets its variables.

## 2. Terraform state bucket (once)

```powershell
cd infra/terraform/bootstrap
terraform init
terraform plan     # expect: Plan: 5 to add, 0 to change, 0 to destroy
terraform apply
terraform output state_bucket
cd ../../..
```

Back up `infra/terraform/bootstrap/terraform.tfstate` somewhere private (it is git-ignored).

## 3. Configure the prod stack

1. In `infra/terraform/envs/prod/versions.tf`, replace `ACCOUNT_ID` in the bucket name with your
   account ID. Commit it: CI needs the real name to run `plan`. (An account ID is not a secret, and
   the repo is private.)
2. Copy `infra/terraform/envs/prod/terraform.tfvars.example` to `terraform.tfvars` (git-ignored) and
   set `alert_email`.
3. `github_repo` defaults to `juank0205/finance`. If the repo has another name, add
   `github_repo = "owner/name"` to `terraform.tfvars`: the deploy role trusts only that repo.

## 4. Create the hosting and the GitHub roles

`enable_web` (default `true`) creates the hosting. It needs an account AWS has verified for
CloudFront; on a new account CloudFront refuses until a Support case clears it, so set
`enable_web = false` in `terraform.tfvars` until then. If the API is already deployed
([deploy-api.md](deploy-api.md)), the plan only adds the `web` module's resources.

The dashboard is served at `dashboard_domain` (`finance.chimborrio.shop`). DNS stays at GoDaddy, so
two CNAME records are added there by hand. (Set `dashboard_domain = ""` to use the bare
`*.cloudfront.net` address instead and skip 4a, 4b and 4d.)

### 4a. The certificate alone

```powershell
cd infra/terraform/envs/prod
terraform init
terraform apply -target='module.web[0].aws_acm_certificate.site[0]'   # 1 to add
# A targeted apply does not save outputs, so read the record from the certificate itself:
terraform state show 'module.web[0].aws_acm_certificate.site[0]' | Select-String resource_record
```

### 4b. Prove the domain to ACM

GoDaddy → **My Products → chimborrio.shop → DNS → Add New Record**:

| Field | Value |
|---|---|
| Type | `CNAME` |
| Name | `resource_record_name` **without** `.chimborrio.shop.` at the end (e.g. `_3f2a….finance`): GoDaddy appends the domain itself |
| Value | `resource_record_value` (`_….acm-validations.aws.`) |
| TTL | default (1 hour) |

Keep this record for good: ACM renews the certificate every year using the same record. Check it is
live (it can take a few minutes):

```powershell
nslookup -type=CNAME <resource_record_name> 1.1.1.1
```

### 4c. Everything else

```powershell
terraform plan     # read it
terraform apply    # waits for the certificate (minutes once 4b's record is live), then CloudFront (~5 minutes)
terraform output
cd ../../..
```

### 4d. Point the subdomain at CloudFront

One more GoDaddy record: type `CNAME`, name `finance`, value `dashboard_dns_records.dashboard.value`
(`d….cloudfront.net`), TTL default.

What it creates:

| Module | Resources | Cost |
|---|---|---|
| `budget` | Monthly budget, emails above $1 actual or forecast | $0 |
| `github` | GitHub OIDC provider, read-only `finance-github-plan` role for CI | $0 |
| `web` | Private bucket (versioned, old versions expire after 30 days), CloudFront with Origin Access Control and security headers, ACM certificate for `dashboard_domain` (us-east-1), `finance-github-deploy-web` role | ~$0 |

It also creates the security baseline (CloudTrail and email alerts, see
[phase-0.md](phase-0.md#5-free-tier-stack-and-dashboard-cd)). Confirm the SNS subscription email it
sends, or the alerts are never delivered.

The deploy role can only list and write that bucket and invalidate that distribution, and only from
workflows running on `main`.

## 5. GitHub repository variables

GitHub → the repo → **Settings → Secrets and variables → Actions → Variables** tab → **New repository
variable**, once per row. Values come from `terraform output github_variables`. These are
identifiers, not credentials, so they are variables, not secrets.

| Name | Value |
|---|---|
| `AWS_DEPLOY_ROLE_ARN` | `github_variables.AWS_DEPLOY_ROLE_ARN` |
| `WEB_BUCKET` | `github_variables.WEB_BUCKET` |
| `WEB_DISTRIBUTION_ID` | `github_variables.WEB_DISTRIBUTION_ID` |
| `AWS_PLAN_ROLE_ARN` | `github_variables.AWS_PLAN_ROLE_ARN` (enables `terraform plan` in CI) |
| `ALERT_EMAIL` | the same email as in `terraform.tfvars` (CI's plan needs it) |
| `VITE_API_URL` | `github_variables.VITE_API_URL` (cloud-api's Function URL) |
| `VITE_COGNITO_DOMAIN` | `github_variables.VITE_COGNITO_DOMAIN` |
| `VITE_COGNITO_CLIENT_ID` | `github_variables.VITE_COGNITO_CLIENT_ID` |

Without the three `VITE_` variables `deploy-dashboard` fails instead of shipping a build on demo
data.

## 6. First deploy

Push the commit from step 3, or **Actions → deploy-dashboard → Run workflow** on `main`. After a
couple of minutes, open `terraform output dashboard_url` (`https://finance.chimborrio.shop`). It
sends you to the Cognito sign-in page and back. The page calls only cloud-api and Cognito: the CSP's
`connect-src` lists exactly those two. Sign-in only works from `dashboard_domain` and localhost, not
from the `*.cloudfront.net` address.

Checks:

```powershell
curl.exe -sI https://finance.chimborrio.shop/ | Select-String "content-security|strict-transport|x-frame"
curl.exe -sI https://<bucket>.s3.eu-west-3.amazonaws.com/index.html   # 403: the bucket is private
```

From then on every push to `main` that changes `frontend/cloud-dashboard/` deploys automatically.

## Day to day

- **Roll back:** Actions → deploy-dashboard → open an older successful run → **Re-run all jobs**. It
  rebuilds and deploys that run's commit.
- **Cost:** expected $0. The budget emails you if the month goes above $1.
- **Who can see it:** anyone with the URL gets the page itself (the build, no data). Every API call
  needs a Cognito sign-in with MFA.
- **Tear down:** `terraform destroy` in `infra/terraform/envs/prod`, then delete the five GitHub
  variables.
