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

```powershell
cd infra/terraform/envs/prod
terraform init
terraform plan     # read it. Expect: Plan: 41 to add, 0 to change, 0 to destroy (16 web/CI + 25 security baseline)
terraform apply    # CloudFront takes about 5 minutes
terraform output
cd ../../..
```

What it creates:

| Module | Resources | Cost |
|---|---|---|
| `budget` | Monthly budget, emails above $1 actual or forecast | $0 |
| `github` | GitHub OIDC provider, read-only `finance-github-plan` role for CI | $0 |
| `web` | Private bucket (versioned, old versions expire after 30 days), CloudFront with Origin Access Control and security headers, `finance-github-deploy-web` role | ~$0 |

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

## 6. First deploy

Push the commit from step 3, or **Actions → deploy-dashboard → Run workflow** on `main`. After a
couple of minutes, open `terraform output dashboard_url`. You should see the dashboard with the
**Demo data** badge.

Checks:

```powershell
curl.exe -sI https://<your-id>.cloudfront.net/ | Select-String "content-security|strict-transport|x-frame"
curl.exe -sI https://<bucket>.s3.eu-west-3.amazonaws.com/index.html   # 403: the bucket is private
```

From then on every push to `main` that changes `frontend/cloud-dashboard/` deploys automatically.

## Day to day

- **Roll back:** Actions → deploy-dashboard → open an older successful run → **Re-run all jobs**. It
  rebuilds and deploys that run's commit.
- **Cost:** expected $0. The budget emails you if the month goes above $1.
- **Who can see it:** anyone with the URL. Fine for demo data; Cognito login comes before any real
  data or the real API.
- **Tear down:** `terraform destroy` in `infra/terraform/envs/prod`, then delete the five GitHub
  variables.
