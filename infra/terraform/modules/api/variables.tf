variable "name" {
  description = "Prefix for resource names"
  type        = string
}

variable "image_digest" {
  description = "sha256:... digest of the cloud-api image in the repository; empty = repository only"
  type        = string
  default     = ""
  validation {
    condition     = var.image_digest == "" || can(regex("^sha256:[0-9a-f]{64}$", var.image_digest))
    error_message = "image_digest must look like sha256:<64 hex characters>."
  }
}

variable "table_name" {
  description = "Ledger table (modules/data)"
  type        = string
}

variable "table_arn" {
  description = "Ledger table ARN (modules/data)"
  type        = string
}

variable "cognito_issuer" {
  description = "Token issuer of the user pool (modules/auth)"
  type        = string
}

variable "cognito_client_id" {
  description = "The dashboard's app client (modules/auth)"
  type        = string
}

variable "reserved_concurrency" {
  description = "Cap on concurrent executions; null = no cap"
  type        = number
  default     = null
}

variable "github_repo" {
  description = "The repo as GitHub's OIDC subject names it: owner@owner_id/name@repo_id"
  type        = string
}

variable "github_oidc_provider_arn" {
  description = "The account's GitHub Actions OIDC provider (modules/github)"
  type        = string
}
