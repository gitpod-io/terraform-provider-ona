resource "terraform_data" "webhook_secret" {
  triggers_replace = var.webhook_secret_version

  provisioner "local-exec" {
    command = "printf '%s\n' \"$WEBHOOK_SECRET\" | install -m 600 /dev/stdin \"$SECRET_FILE_PATH\""

    environment = {
      WEBHOOK_SECRET   = var.webhook_secret
      SECRET_FILE_PATH = var.secret_file_path
    }
  }
}
