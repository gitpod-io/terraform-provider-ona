resource "terraform_data" "secret_target" {
  provisioner "local-exec" {
    command = var.write_command

    environment = {
      ONA_WEBHOOK_SECRET = var.webhook_secret
    }
  }
}
