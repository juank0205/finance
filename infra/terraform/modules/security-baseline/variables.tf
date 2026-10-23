variable "name" {
  description = "Prefix for resource names"
  type        = string
}

variable "alert_email" {
  description = "Receives budget, GuardDuty and root-login alerts (SNS asks you to confirm by email once)"
  type        = string
}

variable "budget_limit_usd" {
  description = "Monthly budget. Alerts at 50% and 100% actual, and 100% forecasted"
  type        = number
  default     = 10
}

variable "create_cost_anomaly_monitor" {
  description = "Newer accounts already have a default services monitor, and only one is allowed. Check Billing > Cost Anomaly Detection first"
  type        = bool
  default     = false
}

variable "log_retention_days" {
  description = "How long CloudTrail logs are kept in S3"
  type        = number
  default     = 365
}

variable "github_repo" {
  description = "owner/repo allowed to assume the CI plan role"
  type        = string
}

variable "state_bucket" {
  description = "Terraform state bucket created by the bootstrap stack"
  type        = string
}

variable "state_kms_alias" {
  description = "KMS alias that encrypts Terraform state"
  type        = string
  default     = "alias/finance-tfstate"
}
