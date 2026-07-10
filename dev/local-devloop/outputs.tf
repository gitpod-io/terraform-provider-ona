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

output "runner_count_from_collection_data_source" {
  description = "Number of runners returned by the collection data source."
  value       = length(data.ona_runners.all.runners)
}
