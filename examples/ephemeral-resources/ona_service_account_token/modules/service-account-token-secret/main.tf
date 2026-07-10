resource "terraform_data" "secret_target" {
  provisioner "local-exec" {
    command = var.write_command

    environment = {
      ONA_SERVICE_ACCOUNT_TOKEN = var.service_account_token
    }
  }
}
