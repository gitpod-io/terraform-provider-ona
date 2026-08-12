#!/usr/bin/env sh

# Legacy service-account import.
terraform import ona_group_membership.terraform_service_account 11111111-1111-4111-8111-111111111111/22222222-2222-4222-8222-222222222222

# The typed service-account form is equivalent to the legacy form above.
# terraform import ona_group_membership.terraform_service_account 11111111-1111-4111-8111-111111111111/service_account/22222222-2222-4222-8222-222222222222

terraform import ona_group_membership.existing_user 11111111-1111-4111-8111-111111111111/user/33333333-3333-4333-8333-333333333333
