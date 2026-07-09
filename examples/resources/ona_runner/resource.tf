resource "ona_runner" "example" {
  name            = "aws-us-east-primary"
  runner_provider = "aws_ec2"

  configuration {
    region                           = "us-east-1"
    release_channel                  = "stable"
    auto_update                      = true
    devcontainer_image_cache_enabled = true
    log_level                        = "info"

    update_window {
      start = "02:00"
      end   = "04:00"
    }
  }
}

output "cloudformation_template_url" {
  value = ona_runner.example.cloudformation_template_url
}
