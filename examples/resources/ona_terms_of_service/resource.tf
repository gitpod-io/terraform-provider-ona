resource "ona_terms_of_service" "example" {
  enabled  = true
  markdown = file("${path.module}/terms.md")
}
