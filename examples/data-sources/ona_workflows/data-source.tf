data "ona_workflows" "enabled" {
  disabled = false
  creator_ids = [
    "<creator-user-id>",
  ]
}

output "workflow_names" {
  value = [for workflow in data.ona_workflows.enabled.workflows : workflow.name]
}
