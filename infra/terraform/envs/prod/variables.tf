variable "region" {
  type    = string
  default = "eu-west-3"
}

variable "alert_email" {
  description = "Where budget and security alerts are sent. Put it in terraform.tfvars (git-ignored)"
  type        = string
}

variable "github_repo" {
  description = "owner/repo whose CI may run terraform plan"
  type        = string
  default     = "juank0205/finance"
}

variable "enable_web" {
  description = "Create the dashboard hosting (S3 + CloudFront). Needs an account AWS has verified for CloudFront"
  type        = bool
  default     = false
}

variable "dashboard_origins" {
  description = "Where the dashboard runs (no trailing slash): allowed to sign in and to call the API. Add the CloudFront address once enable_web is on"
  type        = list(string)
  default     = ["http://localhost:5173", "http://localhost:4173"] # npm run dev, npm run preview
}

variable "cloud_api_image_digest" {
  description = "Digest (sha256:...) of the cloud-api image to run; empty until the first push. See docs/deploy-api.md"
  type        = string
  default     = ""
}
