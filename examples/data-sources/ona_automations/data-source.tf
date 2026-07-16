data "ona_automations" "enabled" {
  disabled = false
  creator_ids = [
    "<creator-user-id>",
  ]
}

output "automation_names" {
  value = [for automation in data.ona_automations.enabled.automations : automation.name]
}
