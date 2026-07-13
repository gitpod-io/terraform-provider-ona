resource "ona_custom_domain" "primary" {
  domain_name      = "ona.example.com"
  cloud_provider   = "aws"
  cloud_account_id = "123456789012"
}
