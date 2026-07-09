resource "ona_security_policy" "baseline" {
  organization_id = "00000000-0000-0000-0000-000000000000"
  name            = "baseline"

  spec {
    block_devices {
      default_effect = "block"
    }
  }
}

resource "ona_organization_policies" "example" {
  organization_id                    = "00000000-0000-0000-0000-000000000000"
  members_require_projects           = true
  members_create_projects            = false
  port_sharing_disabled              = true
  maximum_environment_timeout        = "30m"
  delete_archived_environments_after = "168h"
  security_policy_id                 = ona_security_policy.baseline.id

  agent_policy = {
    mcp_disabled                  = false
    scm_tools_disabled            = false
    command_deny_list             = ["rm -rf /"]
    max_subagents_per_environment = 5
    allowed_agent_ids             = []
  }
}
