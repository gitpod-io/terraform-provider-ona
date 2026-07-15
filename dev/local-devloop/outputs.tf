output "managed_runner_id" {
  description = "ID of the runner managed by this module."
  value       = ona_runner.devloop.runner_id
}

output "cloudformation_template_url" {
  description = "CloudFormation template URL for the managed AWS EC2 runner."
  value       = ona_runner.devloop.cloudformation_template_url
}

output "managed_runner_name_from_data_source" {
  description = "Name of the managed runner read back through the singular data source."
  value       = data.ona_runner.devloop.name
}

output "managed_service_account_id" {
  description = "ID of the service account managed by this module."
  value       = ona_service_account.devloop.id
}

output "managed_group_id" {
  description = "ID of the group managed by this module."
  value       = ona_group.devloop.id
}

output "managed_group_membership_id" {
  description = "ID of the group membership managed by this module."
  value       = ona_group_membership.devloop.id
}

output "managed_organization_role_assignment_id" {
  description = "IDs of the organization role assignments managed by this module, keyed by role."
  value = {
    for role, assignment in ona_organization_role_assignment.devloop : role => assignment.id
  }
}

output "managed_project_id" {
  description = "ID of the project managed by this module."
  value       = ona_project.devloop.id
}

output "managed_webhook_id" {
  description = "ID of the webhook managed by this module."
  value       = ona_webhook.devloop.id
}

output "managed_webhook_url" {
  description = "Generated URL of the webhook managed by this module."
  value       = ona_webhook.devloop.url
}

output "managed_warm_pool_id" {
  description = "ID of the warm pool managed by this module."
  value       = ona_warm_pool.devloop.id
}

output "warm_pool_count_from_collection_data_source" {
  description = "Number of warm pools returned by the collection data source."
  value       = length(data.ona_warm_pools.devloop.warm_pools)
}

output "runner_count_from_collection_data_source" {
  description = "Number of runners returned by the collection data source."
  value       = length(data.ona_runners.all.runners)
}

output "managed_integration_id" {
  description = "ID of the Linear integration managed by this module."
  value       = ona_integration.linear.id
}

output "integration_definition_count" {
  description = "Number of visible integration definitions returned by the data source."
  value       = length(data.ona_integration_definitions.available.definitions)
}
