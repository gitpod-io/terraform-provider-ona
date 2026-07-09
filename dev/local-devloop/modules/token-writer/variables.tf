variable "runner_token" {
  type        = string
  description = "Runner token to write for local testing."
  sensitive   = true
  ephemeral   = true
}

variable "token_file_path" {
  type        = string
  description = "Path where the local test token writer stores the runner token."
  default     = "/tmp/ona-runner-token.txt"
}
