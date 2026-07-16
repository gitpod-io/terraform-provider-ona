data "ona_automations" "enabled" {
  disabled = false
  creator_ids = [
    "<creator-user-id>",
  ]
}

output "workflow_names" {
  value = [for workflow in data.ona_automations.enabled.workflows : workflow.name]
}
