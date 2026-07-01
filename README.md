# Terraform Provider for Ona

This repository hosts the public GitHub Releases for the official Terraform
provider for Ona. The Terraform Registry consumes those release assets to make
provider versions installable with Terraform.

## Usage

```hcl
terraform {
  required_providers {
    ona = {
      source  = "gitpod-io/ona"
      version = "~> 0.1"
    }
  }
}

provider "ona" {}
```

Set `ONA_TOKEN` in the environment or configure authentication as described in
the versioned provider documentation on the Terraform Registry.
