variable "custom_metrics_password" {
  description = "Password or token for the custom metrics pipeline."
  type        = string
  sensitive   = true
}

resource "ona_runner" "aws_primary" {
  name            = "aws-us-east-primary"
  runner_provider = "aws_ec2"

  configuration {
    region                           = "us-east-1"
    release_channel                  = "stable"
    auto_update                      = true
    devcontainer_image_cache_enabled = true
    log_level                        = "info"

    metrics {
      managed {
        enabled = true
      }
    }

    update_window {
      start = "02:00"
      end   = "04:00"
    }
  }
}

# AWS runners return a CloudFormation template URL for runner deployment.
output "aws_cloudformation_template_url" {
  value = ona_runner.aws_primary.cloudformation_template_url
}

# GCP runners do not use CloudFormation, so cloudformation_template_url is null.
resource "ona_runner" "gcp_primary" {
  name            = "gcp-us-central-primary"
  runner_provider = "gcp"

  configuration {
    release_channel                  = "stable"
    auto_update                      = true
    devcontainer_image_cache_enabled = true
    log_level                        = "info"

    metrics {
      custom {
        enabled  = true
        url      = "https://metrics.example.com/api/v1/write"
        username = "runner"
        password = var.custom_metrics_password

        # Change this marker with password to rotate the custom metrics credentials.
        password_version = "1"
      }
    }
  }
}
