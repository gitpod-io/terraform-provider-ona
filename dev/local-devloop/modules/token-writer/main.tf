resource "terraform_data" "runner_token" {
  provisioner "local-exec" {
    command = "printf '%s\n' \"$RUNNER_TOKEN\" > \"$TOKEN_FILE_PATH\""

    environment = {
      RUNNER_TOKEN    = var.runner_token
      TOKEN_FILE_PATH = var.token_file_path
    }
  }
}
