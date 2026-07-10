resource "terraform_data" "registration" {
  provisioner "local-exec" {
    command = var.registration_command

    environment = {
      ONA_RUNNER_TOKEN = var.runner_token
    }
  }
}
