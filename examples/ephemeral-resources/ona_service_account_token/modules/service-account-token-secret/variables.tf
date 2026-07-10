variable "service_account_token" {
  type        = string
  description = "Service-account token to store in an external secret target."
  sensitive   = true
  ephemeral   = true
}

variable "write_command" {
  type        = string
  description = "Command that writes ONA_SERVICE_ACCOUNT_TOKEN from the process environment to an external secret target."
  default     = "printf '%s\n' \"$ONA_SERVICE_ACCOUNT_TOKEN\" >/dev/null"
}
