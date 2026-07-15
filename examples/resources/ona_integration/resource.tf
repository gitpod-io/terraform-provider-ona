data "ona_integration_definitions" "available" {}

locals {
  linear_definition = one([
    for definition in data.ona_integration_definitions.available.definitions : definition
    if definition.host == "linear.app"
  ])
}

resource "ona_integration" "linear" {
  integration_definition_id = local.linear_definition.id
  enabled                   = true
}

variable "custom_mcp_client_secret" {
  type        = string
  sensitive   = true
  nullable    = false
  description = "OAuth client secret for the custom MCP integration."
}

resource "ona_integration" "custom_mcp" {
  name        = "Example MCP server"
  description = "Custom MCP integration managed with Terraform."
  enabled     = true

  capabilities = {
    mcp = {
      url = "https://mcp.example.com/mcp"
    }
  }

  auth = {
    oauth = {
      client_id             = "example-client-id"
      client_secret_version = "v1"
    }
  }

  credentials = {
    oauth_client_secret = var.custom_mcp_client_secret
  }
}
