terraform {
  required_version = ">= 1.10"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }

  # Replace ACCOUNT_ID with the value of `terraform -chdir=../../bootstrap output state_bucket`.
  backend "s3" {
    bucket       = "finance-tfstate-192592197380"
    key          = "prod/terraform.tfstate"
    region       = "eu-west-3"
    encrypt      = true
    use_lockfile = true
  }
}
