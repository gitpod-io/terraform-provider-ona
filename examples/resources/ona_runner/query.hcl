list "ona_runner" "all" {
  provider         = ona
  include_resource = true

  config {
    runner_providers = ["aws_ec2", "gcp"]
  }
}
