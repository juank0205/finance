variable "region" {
  type    = string
  default = "us-east-1"
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
