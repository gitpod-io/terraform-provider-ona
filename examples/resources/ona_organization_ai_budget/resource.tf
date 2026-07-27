resource "ona_organization_ai_budget" "credits" {
  mode                 = "credits"
  monthly_credit_limit = 5000
}

resource "ona_organization_ai_budget" "byok" {
  mode                          = "byok"
  monthly_cost_limit_microunits = 100000000
  currency                      = "usd"
}
