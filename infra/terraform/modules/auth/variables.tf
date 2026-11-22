variable "name" {
  description = "Prefix for resource names"
  type        = string
}

variable "callback_urls" {
  description = "Where the hosted sign-in page may send the browser back to (the dashboard's origins)"
  type        = list(string)
}
