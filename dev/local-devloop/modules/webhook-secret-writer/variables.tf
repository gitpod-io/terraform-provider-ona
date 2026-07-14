variable "webhook_secret" {
  type        = string
  description = "Webhook signing secret to write for local testing."
  sensitive   = true
  ephemeral   = true
}

variable "webhook_secret_version" {
  type        = string
  description = "Non-sensitive rotation marker used to replace the local writer resource."
}

variable "secret_file_path" {
  type        = string
  description = "Path where the local test writer stores the webhook signing secret."
  default     = "/tmp/ona-webhook-secret.txt"
}
