variable "name" {
  description = "Prefix for resource names"
  type        = string
}

variable "alert_email" {
  description = "Receives security alerts (SNS asks you to confirm by email once) and cost anomalies"
  type        = string
}

variable "enable_guardduty" {
  description = "Threat detection. Free for 30 days, then billed by analysed events (roughly $1-4/month here)"
  type        = bool
  default     = false
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
