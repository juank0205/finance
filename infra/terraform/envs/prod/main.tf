provider "aws" {
  region = var.region
  default_tags {
    tags = {
      Project   = "finance"
      ManagedBy = "terraform"
      Env       = "prod"
    }
  }
}

# Global pieces that only live in us-east-1: IAM and root sign-in events (security baseline), and
# CloudFront certificates for a custom domain later.
provider "aws" {
  alias  = "us_east_1"
  region = "us-east-1"
  default_tags {
    tags = {
      Project   = "finance"
      ManagedBy = "terraform"
      Env       = "prod"
    }
  }
}

locals {
  name = "finance"
  # Where the dashboard may sign in from and call the API: the domain once hosting is on, plus local runs.
  dashboard_origins = concat(
    var.enable_web && var.dashboard_domain != "" ? ["https://${var.dashboard_domain}"] : [],
    var.dashboard_origins,
  )
}

# Uncomment each module as its phase is built. Everything enabled here stays inside the free tier.

module "budget" {
  source      = "../../modules/budget"
  name        = local.name
  alert_email = var.alert_email
}

module "github" {
  source      = "../../modules/github"
  name        = local.name
  github_repo = var.github_repo
}

# Dashboard hosting (S3 + CloudFront). Needs an account AWS has verified for CloudFront; without it
# the dashboard runs locally against the real API (see frontend/cloud-dashboard/README.md).
module "web" {
  source                   = "../../modules/web"
  count                    = var.enable_web ? 1 : 0
  name                     = local.name
  github_repo              = var.github_repo
  github_oidc_provider_arn = module.github.oidc_provider_arn
  domain_name              = var.dashboard_domain
  providers = {
    aws           = aws
    aws.us_east_1 = aws.us_east_1
  }
  # The CSP lets the page call exactly these: cloud-api's Function URL and Cognito's token endpoint.
  connect_src = concat(
    module.api.url == null ? [] : [trimsuffix(module.api.url, "/")],
    [module.auth.hosted_ui_url],
  )
}

# CloudTrail, root-activity and sensitive-change alerts, account hardening. ~$0.10/month (CloudTrail's
# S3 writes); GuardDuty stays off unless enable_guardduty = true.
module "security_baseline" {
  source      = "../../modules/security-baseline"
  name        = local.name
  alert_email = var.alert_email
  providers = {
    aws           = aws
    aws.us_east_1 = aws.us_east_1
  }
}

# The ledger table (DynamoDB). $0 inside the always-free allowance; backups well below $0.01/month.
module "data" {
  source = "../../modules/data"
  name   = local.name
}

# module "iot" {                 # phase 4
#   source = "../../modules/iot"
#   name   = local.name
# }

# Dashboard sign-in (Cognito, no self sign-up, TOTP MFA). $0 in the Lite tier.
module "auth" {
  source        = "../../modules/auth"
  name          = local.name
  callback_urls = [for o in local.dashboard_origins : "${o}/"]
}

# cloud-api on Lambda behind a public Function URL that checks Cognito tokens. $0 inside Lambda's
# always-free allowance; ECR storage about $0.003/month.
module "api" {
  source            = "../../modules/api"
  name              = local.name
  image_digest      = var.cloud_api_image_digest
  table_name        = module.data.table_name
  table_arn         = module.data.table_arn
  cognito_issuer    = module.auth.issuer
  cognito_client_id = module.auth.client_id
  allowed_origins   = local.dashboard_origins

  github_repo              = var.github_repo
  github_oidc_provider_arn = module.github.oidc_provider_arn
}

moved {
  from = module.web
  to   = module.web[0]
}

# What the dashboard needs in frontend/cloud-dashboard/.env.local (none of it is a secret).
output "dashboard_env" {
  value = {
    VITE_API_URL           = module.api.url
    VITE_COGNITO_DOMAIN    = module.auth.hosted_ui_url
    VITE_COGNITO_CLIENT_ID = module.auth.client_id
  }
}

output "cloud_api_repository" {
  value = module.api.repository_url
}

output "user_pool_id" {
  value = module.auth.user_pool_id
}

output "dashboard_url" {
  value = one(module.web[*].url)
}

# The two CNAME records to add at the registrar (GoDaddy's Name field takes the part before the
# domain). The certificate's appears after the first, certificate-only apply.
output "dashboard_dns_records" {
  value = var.enable_web && var.dashboard_domain != "" ? {
    certificate = one(module.web[*].certificate_validation)
    dashboard = {
      name  = var.dashboard_domain
      value = one(module.web[*].cloudfront_domain)
    }
  } : null
}

# The values to set as GitHub repository variables (none of them is a secret). The web ones are
# null while enable_web is false; the VITE_ ones are what deploy-dashboard builds the dashboard with.
output "github_variables" {
  value = {
    AWS_PLAN_ROLE_ARN       = module.github.plan_role_arn
    AWS_API_DEPLOY_ROLE_ARN = module.api.deploy_role_arn
    AWS_DEPLOY_ROLE_ARN     = one(module.web[*].deploy_role_arn)
    WEB_BUCKET              = one(module.web[*].bucket)
    WEB_DISTRIBUTION_ID     = one(module.web[*].distribution_id)
    VITE_API_URL            = module.api.url
    VITE_COGNITO_DOMAIN     = module.auth.hosted_ui_url
    VITE_COGNITO_CLIENT_ID  = module.auth.client_id
  }
}
