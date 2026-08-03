resource "ona_team_ai_budget" "platform_credits" {
  team_id       = "44444444-4444-4444-8444-444444444444"
  mode          = "credits"
  credit_budget = 1500
}

resource "ona_team_ai_budget" "platform_byok" {
  team_id                = "44444444-4444-4444-8444-444444444444"
  mode                   = "byok"
  cost_budget_microunits = 50000000
  cost_budget_currency   = "usd"
}
