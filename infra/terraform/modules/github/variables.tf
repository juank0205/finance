variable "name" {
  description = "Prefix for resource names"
  type        = string
}

variable "github_repo" {
  description = "The repo as GitHub's OIDC subject names it: owner@owner_id/name@repo_id"
  type        = string
}
