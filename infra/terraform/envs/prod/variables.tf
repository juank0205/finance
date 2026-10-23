variable "region" {
  type    = string
  default = "us-east-1"
}

variable "alert_email" {
  description = "Where budget and security alerts are sent"
  type        = string
  default     = ""
}
