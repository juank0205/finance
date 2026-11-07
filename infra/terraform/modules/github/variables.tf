variable "name" {
  description = "Prefix for resource names"
  type        = string
}

variable "github_repo" {
  description = "owner/repo allowed to assume the CI roles"
  type        = string
}
