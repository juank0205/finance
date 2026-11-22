# Deploying the API (and running the dashboard against it)

Everything except the dashboard hosting lives in AWS: the ledger table, Cognito sign-in and
`cloud-api` on Lambda behind a Function URL. The dashboard runs on your machine
(`http://localhost:5173`) until AWS enables CloudFront for the account; then `enable_web = true`
moves it to CloudFront too.

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
function's IAM role, but no function yet. If an earlier, failed apply left pieces of the web module
(S3 bucket, deploy role), the plan destroys them, since `enable_web` is false; they come back later.

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

Repeat steps 3 and 4 with the new digest. The repository keeps the last three images, so a rollback
is applying an older digest.

## What protects it

- The Function URL is public, but every `/v1` request needs a Cognito access token for this pool and
  this app client (`backend/internal/auth`). `cloud-api` refuses to start in Lambda without the
  Cognito settings, so a missing variable cannot open the ledger. Only `/healthz` answers without one.
- The pool has no self sign-up and requires TOTP MFA.
- The function's role can only read and write items in the ledger table and write its own logs.
- CORS lets only the local dashboard origins call it from a browser. CORS does not stop scripts or
  curl; the token check does.
- There is no rate limit in front of the function. Lambda's always-free allowance (1M requests a month)
  plus the $5 budget alarm are the cost guard until reserved concurrency is possible
  ([security.md](security.md)).
