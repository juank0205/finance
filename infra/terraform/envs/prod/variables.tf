variable "region" {
  type    = string
  default = "eu-west-3"
}

variable "alert_email" {
  description = "Where budget and security alerts are sent. Put it in terraform.tfvars (git-ignored)"
  type        = string
}

variable "github_repo" {
  # GitHub's OIDC subject names the repo with its owner's and its own numeric IDs. They never change,
  # so a repo that later takes over this name cannot assume the roles.
  description = "The repo whose CI may assume the AWS roles, as owner@owner_id/name@repo_id (GitHub's OIDC subject)"
  type        = string
  default     = "juank0205@58149552/finance@1431573979"
  validation {
    condition     = can(regex("^[^/@]+@[0-9]+/[^/@]+@[0-9]+$", var.github_repo))
    error_message = "github_repo must look like owner@owner_id/name@repo_id."
  }
}

variable "enable_web" {
  description = "Create the dashboard hosting (S3 + CloudFront). Needs an account AWS has verified for CloudFront"
  type        = bool
  default     = true
}

variable "dashboard_domain" {
  description = "Where CloudFront serves the dashboard (DNS at the registrar, see docs/deploy-dashboard.md); empty = the *.cloudfront.net address"
  type        = string
  default     = "finance.chimborrio.shop"
}

variable "dashboard_origins" {
  description = "Other places the dashboard runs (no trailing slash), allowed to sign in and to call the API besides https://<dashboard_domain>"
  type        = list(string)
  default     = ["http://localhost:5173", "http://localhost:4173"] # npm run dev, npm run preview
}

variable "cloud_api_image_digest" {
  description = "Digest (sha256:...) of the cloud-api image to run; empty until the first push. See docs/deploy-api.md"
  type        = string
  default     = ""
}
