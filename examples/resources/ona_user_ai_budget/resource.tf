resource "ona_user_ai_budget" "developer_credits" {
  user_id              = "22222222-2222-4222-8222-222222222222"
  mode                 = "credits"
  monthly_credit_limit = 750
}

resource "ona_user_ai_budget" "automation_byok_exemption" {
  user_id = "33333333-3333-4333-8333-333333333333"
  mode    = "byok"
  no_cap  = true
}
