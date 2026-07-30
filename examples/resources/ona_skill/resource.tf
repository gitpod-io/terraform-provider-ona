resource "ona_skill" "security_review" {
  name        = "Security review"
  description = "Review changes against the organization's security checklist."
  prompt      = file("${path.module}/security-review.md")
  command     = "security-review"
}
