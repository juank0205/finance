variable "name" {
  description = "Prefix for resource names"
  type        = string
}

variable "github_repo" {
  description = "The repo as GitHub's OIDC subject names it: owner@owner_id/name@repo_id"
  type        = string
}

variable "github_oidc_provider_arn" {
  description = "The account's GitHub Actions OIDC provider (modules/github)"
  type        = string
}

variable "connect_src" {
  description = "Origins the dashboard may call besides its own (cloud-api, Cognito), for the CSP"
  type        = list(string)
  default     = []
}

variable "domain_name" {
  description = "Where the dashboard is served, e.g. finance.example.com; empty = the *.cloudfront.net address"
  type        = string
  default     = ""
}
