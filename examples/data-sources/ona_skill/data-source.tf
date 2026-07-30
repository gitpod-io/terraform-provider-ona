data "ona_skill" "security_review" {
  skill_id = "00000000-0000-0000-0000-000000000000"
}

output "security_review_name" {
  value = data.ona_skill.security_review.name
}

output "security_review_prompt" {
  value = data.ona_skill.security_review.prompt
}
