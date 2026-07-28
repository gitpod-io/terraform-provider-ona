#!/usr/bin/env sh

# Both mode-specific resources may import dimensions of the same shared allocation.
terraform import ona_team_ai_budget.platform_credits 11111111-1111-4111-8111-111111111111/44444444-4444-4444-8444-444444444444/credits
terraform import ona_team_ai_budget.platform_byok 11111111-1111-4111-8111-111111111111/44444444-4444-4444-8444-444444444444/byok
