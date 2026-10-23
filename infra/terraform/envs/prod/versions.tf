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
    bucket       = "finance-tfstate-ACCOUNT_ID"
    key          = "prod/terraform.tfstate"
    region       = "us-east-1"
    encrypt      = true
    kms_key_id   = "alias/finance-tfstate"
    use_lockfile = true
  }
}
