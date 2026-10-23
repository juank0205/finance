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

data "aws_caller_identity" "current" {}

locals {
  name = "finance"
}

# Uncomment each module as its phase is built.

module "security_baseline" { # phase 0
  source       = "../../modules/security-baseline"
  name         = local.name
  alert_email  = var.alert_email
  github_repo  = var.github_repo
  state_bucket = "finance-tfstate-${data.aws_caller_identity.current.account_id}"
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

output "github_plan_role_arn" {
  description = "Set as the AWS_PLAN_ROLE_ARN variable in the GitHub repo"
  value       = module.security_baseline.github_plan_role_arn
}
