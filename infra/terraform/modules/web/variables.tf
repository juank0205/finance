variable "name" {
  description = "Prefix for resource names"
  type        = string
}

variable "github_repo" {
  description = "owner/repo whose main branch may deploy the dashboard"
  type        = string
}

variable "github_oidc_provider_arn" {
  description = "The account's GitHub Actions OIDC provider (modules/github)"
  type        = string
}
