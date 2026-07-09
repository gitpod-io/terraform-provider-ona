package v1

// WatchRequestType returns a stable label for a WatchRequests response payload.
func WatchRequestType(request any) string {
	switch request.(type) {
	case *WatchRequestsResponse_CallPing:
		return "call_ping"
	case *WatchRequestsResponse_CallParseContext:
		return "call_parse_context"
	case *WatchRequestsResponse_CallCheckAuthenticationForHost:
		return "call_check_authentication_for_host"
	case *WatchRequestsResponse_CallCheckRepositoryAccess:
		return "call_check_repository_access"
	case *WatchRequestsResponse_CallSendMessageToAgentExecution:
		return "call_send_message_to_agent_execution"
	case *WatchRequestsResponse_CallImprovePromptForAgent:
		return "call_improve_prompt_for_agent"
	case *WatchRequestsResponse_CallSearchRepositories:
		return "call_search_repositories"
	case *WatchRequestsResponse_CallListScmOrganizations:
		return "call_list_scm_organizations"
	case *WatchRequestsResponse_CallValidateConfig:
		return "call_validate_config"
	case *WatchRequestsResponse_EventEnvironmentSpecChange:
		return "event_environment_spec_change"
	case *WatchRequestsResponse_EventEnvironmentMarkedActive:
		return "event_environment_marked_active"
	case *WatchRequestsResponse_EventScmIntegrationChange:
		return "event_scm_integration_change"
	case *WatchRequestsResponse_EventIntegrationChange:
		return "event_integration_change"
	case *WatchRequestsResponse_EventLlmIntegrationChange:
		return "event_llm_integration_change"
	case *WatchRequestsResponse_EventHostAuthenticationTokenDeleted:
		return "event_host_authentication_token_deleted"
	case *WatchRequestsResponse_EventRunnerConfigurationChange:
		return "event_runner_configuration_change"
	case *WatchRequestsResponse_EventAgentExecutionSpecChange:
		return "event_agent_execution_spec_change"
	case *WatchRequestsResponse_EventSnapshotSpecChange:
		return "event_snapshot_spec_change"
	case *WatchRequestsResponse_EventWarmPoolSpecChange:
		return "event_warm_pool_spec_change"
	default:
		return "unknown"
	}
}
