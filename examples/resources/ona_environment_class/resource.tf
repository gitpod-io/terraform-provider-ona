resource "ona_environment_class" "large" {
  runner_id = ona_runner.aws_primary.runner_id

  display_name = "Large (8 vCPU / 32 GB)"
  description  = "High-memory class for monorepo builds"

  configuration = {
    machineType = "m6i.2xlarge"
    diskSizeGb  = "100"
  }
}
