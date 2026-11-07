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

module "web" {
  source                   = "../../modules/web"
  name                     = local.name
  github_repo              = var.github_repo
  github_oidc_provider_arn = module.github.oidc_provider_arn
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

# module "data" {                # phase 4
#   source = "../../modules/data"
#   name   = local.name
# }

# module "iot" {                 # phase 4
#   source = "../../modules/iot"
#   name   = local.name
# }

# module "api" {                 # phase 5
#   source = "../../modules/api"
#   name   = local.name
# }

# module "web" {                 # phase 5
#   source = "../../modules/web"
#   name   = local.name
# }

output "dashboard_url" {
  value = module.web.url
}

# The four values to set as GitHub repository variables (none of them is a secret).
output "github_variables" {
  value = {
    AWS_PLAN_ROLE_ARN   = module.github.plan_role_arn
    AWS_DEPLOY_ROLE_ARN = module.web.deploy_role_arn
    WEB_BUCKET          = module.web.bucket
    WEB_DISTRIBUTION_ID = module.web.distribution_id
  }
}
