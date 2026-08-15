terraform {
  required_providers {
    ona = {
      source  = "gitpod-io/ona"
      version = "= 0.3.0-beta.48"
    }
  }
}

# Set ONA_TOKEN in the environment before running Terraform.
provider "ona" {
  # Rate-limit retries default to 5 attempts with a 30-second delay cap.
  # Set rate_limit_max_retries = 0 to disable them.
  # rate_limit_max_retries     = 5
  # rate_limit_max_retry_delay = "30s"
}

# Optional: set host only when using a non-default Ona API host.
# provider "ona" {
#   host = "https://<ona-hostname>"
# }
