resource "ona_organization_policies" "current" {
  members_require_projects           = true
  members_create_projects            = false
  port_sharing_disabled              = true
  maximum_environment_timeout        = "30m"
  delete_archived_environments_after = "168h"
  agent_policy = {
    mcp_disabled                  = false
    scm_tools_disabled            = false
    command_deny_list             = ["git push --force"]
    conversation_sharing_policy   = "organization"
    max_subagents_per_environment = 5
    allowed_agent_ids             = []
    codex_model_states = {
      CODEX_OPEN_AI_MODEL_GPT_5_5     = "disabled"
      CODEX_OPEN_AI_MODEL_GPT_5_6_SOL = "allowed"
    }
  }
}
