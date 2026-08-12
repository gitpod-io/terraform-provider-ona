variable "automation_id" {
  type        = string
  description = "ID of an existing Ona Automation."
}

resource "ona_group" "automation_executors" {
  name        = "Automation Executors"
  description = "Service accounts and users allowed to run the Automation."
}

resource "ona_automation_role_assignment" "executors" {
  automation_id = var.automation_id
  group_id      = ona_group.automation_executors.id
  role          = "executor"
}
