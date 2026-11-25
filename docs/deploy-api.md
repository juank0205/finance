# Deploying the API (and running the dashboard against it)

The ledger table, Cognito sign-in and `cloud-api` on Lambda behind a Function URL. The dashboard is
served from CloudFront ([deploy-dashboard.md](deploy-dashboard.md)) and can also run on your machine
(`http://localhost:5173`) against the same API.

All commands are PowerShell, run from the repository root unless a `cd` says otherwise.

## 0. Sign in

```powershell
aws sso login --sso-session admin
$env:AWS_PROFILE = "admin-finance-prod"
```

## 1. Table, user pool and image repository

```powershell
cd infra/terraform/envs/prod
terraform init                      # downloads the provider pinned in .terraform.lock.hcl
terraform plan -out prod.tfplan
terraform apply prod.tfplan
```

Read the plan before applying. With `cloud_api_image_digest` still empty it creates the DynamoDB
table, the Cognito user pool, domain and app client, the ECR repository, the log group and the
function's IAM role, but no function yet. It also creates the dashboard hosting unless
`enable_web = false` is in `terraform.tfvars` (needed while AWS has not verified the account for
CloudFront).

## 2. Your user

The pool has no sign-up page. Create your own user once; Cognito emails a temporary password.

```powershell
$pool  = terraform output -raw user_pool_id
$email = "you@example.com"
aws cognito-idp admin-create-user --user-pool-id $pool --username $email `
  --user-attributes Name=email,Value=$email Name=email_verified,Value=true `
  --desired-delivery-mediums EMAIL
```

At the first sign-in the hosted page asks for a new password (14+ characters) and to set up an
authenticator app (TOTP). MFA cannot be skipped.

## 3. Build and push the image

The Docker daemon must be running. Tags are immutable, so each push gets a new one.

`docker login` with an ECR token fails on Windows ("The stub received bad data": the token is too
long for the Windows credential store). Use AWS's ECR credential helper instead: Docker asks it for a
short-lived token from your SSO session on every push, and nothing is stored. A separate Docker
config folder keeps this away from your normal Docker settings.

```powershell
# once: the helper (lands in $HOME\go\bin)
go install github.com/awslabs/amazon-ecr-credential-helper/ecr-login/cli/docker-credential-ecr-login@latest

$repo = terraform output -raw cloud_api_repository
$env:PATH = "$HOME\go\bin;$env:PATH"
$env:AWS_REGION = "eu-west-3"
$env:DOCKER_CONFIG = "$HOME\.docker-ecr"
New-Item -ItemType Directory -Force $env:DOCKER_CONFIG | Out-Null
Set-Content "$env:DOCKER_CONFIG\config.json" ('{"credHelpers":{"' + $repo.Split('/')[0] + '":"ecr-login"}}')

cd ../../../../backend
$tag = Get-Date -Format "yyyyMMdd-HHmmss"
# --provenance/--sbom off: Lambda rejects the multi-manifest index buildx creates otherwise.
docker buildx build --platform linux/arm64 --target lambda --build-arg CMD=cloud-api `
  --provenance=false --sbom=false -t "${repo}:$tag" --push .

$digest = aws ecr describe-images --repository-name finance/cloud-api --image-ids imageTag=$tag `
  --query "imageDetails[0].imageDigest" --output text
$digest                             # sha256:...
```

## 4. Run it on Lambda

Put the digest in `infra/terraform/envs/prod/terraform.tfvars` (git-ignored):

```hcl
cloud_api_image_digest = "sha256:..."
```

```powershell
cd ../infra/terraform/envs/prod
terraform plan -out prod.tfplan     # creates the function, its URL and the two URL permissions
terraform apply prod.tfplan
terraform output dashboard_env
```

Check it: without a token the API must refuse.

```powershell
$url = (terraform output -json dashboard_env | ConvertFrom-Json).VITE_API_URL
curl.exe -i "${url}healthz"         # 200 {"status":"ok"}
curl.exe -i "${url}v1/summary"      # 401 {"error":"sign in required"}
```

## 5. The dashboard

Copy the three `dashboard_env` values into `frontend/cloud-dashboard/.env.local`, then:

```powershell
cd frontend/cloud-dashboard
npm run dev                         # http://localhost:5173
```

Use `localhost`, not `127.0.0.1`: sign-in and CORS only accept the origins in `dashboard_origins`.

## Updating the API

CI deploys it: every push to `main` runs the `deploy-api` job in `ci` once the `backend` job passes.
It builds the image (tagged with the git hash of `backend/`, so pushes that leave the backend alone
reuse the existing image), pushes it, moves the function to it and checks that `/healthz` answers
200 and `/v1/summary` without a token 401. If either check fails, the function goes back to the
previous image and the job fails.

The digest in `terraform.tfvars` only picks the image the function is created with. Terraform
ignores the image afterwards, so `terraform apply` never undoes a CI deploy.

### Turning it on (once)

1. Create the deploy role. It can only push to `finance/cloud-api` and point `finance-cloud-api` at
   a new image, and only workflows on `main` can assume it.

   ```powershell
   cd infra/terraform/envs/prod
   terraform plan -out prod.tfplan     # expect: 2 to add (finance-github-deploy-api + its policy)
   terraform apply prod.tfplan
   terraform output github_variables   # AWS_API_DEPLOY_ROLE_ARN
   ```

2. GitHub → the repo → **Settings → Secrets and variables → Actions → Variables** → **New
   repository variable**: `AWS_API_DEPLOY_ROLE_ARN` = the value above. It is not a secret.

### Rolling back

Revert the commit on `main`; CI deploys the previous code. In a hurry, from your machine (the
repository keeps the last three images):

```powershell
aws ecr describe-images --repository-name finance/cloud-api --query "imageDetails[].[imagePushedAt,imageDigest]" --output table
aws lambda update-function-code --function-name finance-cloud-api --image-uri "<repository>@sha256:..."
```

The next push to `main` deploys `main` again.

## What protects it

- The Function URL is public, but every `/v1` request needs a Cognito access token for this pool and
  this app client (`backend/internal/auth`). `cloud-api` refuses to start in Lambda without the
  Cognito settings, so a missing variable cannot open the ledger. Only `/healthz` answers without one.
- The pool has no self sign-up and requires TOTP MFA.
- The function's role can only read and write items in the ledger table and write its own logs.
- CI's deploy role (`finance-github-deploy-api`) can push images and change the function's code, not
  its role, settings or URL. Its trust policy only accepts workflows running on `main`.
- CORS lets only the local dashboard origins call it from a browser. CORS does not stop scripts or
  curl; the token check does.
- There is no rate limit in front of the function. Lambda's always-free allowance (1M requests a month)
  plus the $5 budget alarm are the cost guard until reserved concurrency is possible
  ([security.md](security.md)).
