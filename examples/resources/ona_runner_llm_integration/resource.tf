resource "ona_runner_llm_integration" "bedrock_mantle" {
  runner_id = ona_runner.aws_primary.runner_id

  models = [
    "sonnet_4",
    "sonnet_4_extended",
  ]

  endpoint        = "https://bedrock-mantle.us-east-1.api.aws"
  api_key         = var.bedrock_mantle_token
  api_key_version = "2026-07-14"
  max_tokens      = 8000
}

resource "ona_runner_llm_integration" "openai_compatible" {
  runner_id = ona_runner.aws_primary.runner_id

  models = [
    "openai_auto",
  ]

  endpoint        = "https://api.openai.com/v1"
  api_key         = var.openai_api_key
  api_key_version = "2026-07-14"
}
