resource "ona_service_account" "ci" {
  name        = "ci-pipeline"
  description = "CI/CD Pipeline automation"
  valid_until = "2099-01-01T00:00:00Z"
}
