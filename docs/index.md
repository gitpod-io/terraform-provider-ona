---
page_title: "ona Provider"
description: |-
  The Ona provider manages Ona resources with Terraform.
---

# ona Provider

The Ona provider manages Ona resources with Terraform.

## Example Usage

```terraform
terraform {
  required_providers {
    ona = {
      source = "gitpod-io/ona"
    }
  }
}

provider "ona" {}
```

Version-specific provider, resource, and data source documentation is published
from the release documentation prepared with each provider version.
