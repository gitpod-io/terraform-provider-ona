terraform {
  required_providers {
    ona = {
      source  = "gitpod-io/ona"
      version = "= 0.2.0-beta.2"
    }
  }
}

# Set ONA_TOKEN in the environment before running Terraform.
provider "ona" {}

# Optional: set host only when using a non-default Ona API host.
# provider "ona" {
#   host = "https://<ona-hostname>"
# }
