variable "webhook_secret" {
  type        = string
  description = "Webhook signing secret to store in an external secret target."
  sensitive   = true
  ephemeral   = true
}

variable "write_command" {
  type        = string
  description = "Command that writes ONA_WEBHOOK_SECRET from the process environment to an external secret target."
  default     = "printf '%s\n' \"$ONA_WEBHOOK_SECRET\" >/dev/null"
}
