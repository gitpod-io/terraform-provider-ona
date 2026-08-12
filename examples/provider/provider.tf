terraform {
  required_providers {
    ona = {
      source  = "gitpod-io/ona"
      version = "= 0.3.0-beta.48"
    }
  }
}

# Set ONA_TOKEN in the environment before running Terraform.
provider "ona" {}

# Optional: set host only when using a non-default Ona API host.
# provider "ona" {
#   host = "https://<ona-hostname>"
# }
