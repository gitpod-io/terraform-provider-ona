variable "runner_token" {
  type        = string
  description = "Runner registration token."
  sensitive   = true
  ephemeral   = true
}

variable "registration_command" {
  type        = string
  description = "Command that registers the runner using ONA_RUNNER_TOKEN from the process environment."
  default     = "printf '%s\n' \"$ONA_RUNNER_TOKEN\" >/dev/null"
}
