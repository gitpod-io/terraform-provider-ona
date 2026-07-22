// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package organization

import (
	"context"
	"fmt"
	"sort"
	"time"

	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/durationpb"
)

func updatePoliciesRequestFromConfig(ctx context.Context, organizationID string, plan PoliciesModel, cfg tfsdk.Config, current *v1.OrganizationPolicies) (*v1.UpdateOrganizationPoliciesRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	req := &v1.UpdateOrganizationPoliciesRequest{
		OrganizationId: organizationID,
	}

	if value, ok := durationFromConfig(ctx, cfg, path.Root("maximum_environment_timeout"), &diags); ok {
		req.MaximumEnvironmentTimeout = value
	}
	if value, ok := boolFromConfig(ctx, cfg, path.Root("members_require_projects"), plan.MembersRequireProjects, &diags); ok {
		req.MembersRequireProjects = value
	}
	if value, ok := boolFromConfig(ctx, cfg, path.Root("members_create_projects"), plan.MembersCreateProjects, &diags); ok {
		req.MembersCreateProjects = value
	}
	if !plan.AllowedEditorIDs.IsNull() && !plan.AllowedEditorIDs.IsUnknown() {
		req.AllowedEditorIds = stringSliceFromSet(ctx, plan.AllowedEditorIDs, &diags)
	}
	if value, ok := stringFromConfig(ctx, cfg, path.Root("default_editor_id"), plan.DefaultEditorID, &diags); ok {
		req.DefaultEditorId = value
	}
	if value, ok := boolFromConfig(ctx, cfg, path.Root("allow_local_runners"), plan.AllowLocalRunners, &diags); ok {
		req.AllowLocalRunners = value
	}
	if value, ok := int64FromConfig(ctx, cfg, path.Root("maximum_running_environments_per_user"), plan.MaximumRunningEnvironmentsPerUser, &diags); ok {
		req.MaximumRunningEnvironmentsPerUser = value
	}
	if value, ok := int64FromConfig(ctx, cfg, path.Root("maximum_environments_per_user"), plan.MaximumEnvironmentsPerUser, &diags); ok {
		req.MaximumEnvironmentsPerUser = value
	}
	if value, ok := stringFromConfig(ctx, cfg, path.Root("default_environment_image"), plan.DefaultEnvironmentImage, &diags); ok {
		req.DefaultEnvironmentImage = value
	}
	if value, ok := boolFromConfig(ctx, cfg, path.Root("port_sharing_disabled"), plan.PortSharingDisabled, &diags); ok {
		req.PortSharingDisabled = value
	}
	if value, ok := durationFromConfig(ctx, cfg, path.Root("delete_archived_environments_after"), &diags); ok {
		req.DeleteArchivedEnvironmentsAfter = value
	}
	if value, ok := durationFromConfig(ctx, cfg, path.Root("maximum_environment_lifetime"), &diags); ok {
		req.MaximumEnvironmentLifetime = value
	}
	if value, ok := boolFromConfig(ctx, cfg, path.Root("require_custom_domain_access"), plan.RequireCustomDomainAccess, &diags); ok {
		req.RequireCustomDomainAccess = value
	}
	if plan.EditorVersionRestrictions != nil {
		req.EditorVersionRestrictions = editorVersionRestrictionsFromModel(ctx, plan.EditorVersionRestrictions, &diags)
	}
	if value, ok := boolFromConfig(ctx, cfg, path.Root("restrict_account_creation_to_scim"), plan.RestrictAccountCreationToSCIM, &diags); ok {
		req.RestrictAccountCreationToScim = value
	}
	if value, ok := boolFromConfig(ctx, cfg, path.Root("web_browser_disabled"), plan.WebBrowserDisabled, &diags); ok {
		req.WebBrowserDisabled = value
	}
	if value, ok := boolFromConfig(ctx, cfg, path.Root("disable_from_scratch"), plan.DisableFromScratch, &diags); ok {
		req.DisableFromScratch = value
	}
	if value, ok := stringFromConfig(ctx, cfg, path.Root("security_policy_id"), plan.SecurityPolicyID, &diags); ok {
		req.SecurityPolicyId = value
	}
	if value, ok := durationFromConfig(ctx, cfg, path.Root("archive_environments_after"), &diags); ok {
		req.ArchiveEnvironmentsAfter = value
	}
	if plan.AgentPolicy != nil {
		validateAgentPolicy(plan.AgentPolicy, &diags)
		req.AgentPolicy = agentPolicyUpdateFromModel(ctx, plan.AgentPolicy, current.GetAgentPolicy(), &diags)
	}
	if diags.HasError() {
		return nil, diags
	}
	return req, diags
}

func populatePoliciesModel(ctx context.Context, data *PoliciesModel, policies *v1.OrganizationPolicies, prior PoliciesModel, populateUnmanaged bool) diag.Diagnostics {
	var diags diag.Diagnostics
	if policies == nil {
		diags.AddError("Missing Organization Policies", "The Ona API returned an empty organization policy object.")
		return diags
	}

	data.ID = types.StringValue(policies.GetOrganizationId())
	data.MaximumEnvironmentTimeout = durationValue(policies.GetMaximumEnvironmentTimeout(), prior.MaximumEnvironmentTimeout)
	data.MembersRequireProjects = types.BoolValue(policies.GetMembersRequireProjects())
	data.MembersCreateProjects = types.BoolValue(policies.GetMembersCreateProjects())
	data.AllowedEditorIDs = stringSetValue(ctx, policies.GetAllowedEditorIds(), prior.AllowedEditorIDs, populateUnmanaged, &diags)
	data.DefaultEditorID = types.StringValue(policies.GetDefaultEditorId())
	data.AllowLocalRunners = types.BoolValue(policies.GetAllowLocalRunners())
	data.MaximumRunningEnvironmentsPerUser = types.Int64Value(policies.GetMaximumRunningEnvironmentsPerUser())
	data.MaximumEnvironmentsPerUser = types.Int64Value(policies.GetMaximumEnvironmentsPerUser())
	data.DefaultEnvironmentImage = types.StringValue(policies.GetDefaultEnvironmentImage())
	data.PortSharingDisabled = types.BoolValue(policies.GetPortSharingDisabled())
	data.DeleteArchivedEnvironmentsAfter = durationValue(policies.GetDeleteArchivedEnvironmentsAfter(), prior.DeleteArchivedEnvironmentsAfter)
	data.MaximumEnvironmentLifetime = durationValue(policies.GetMaximumEnvironmentLifetime(), prior.MaximumEnvironmentLifetime)
	data.RequireCustomDomainAccess = types.BoolValue(policies.GetRequireCustomDomainAccess())
	data.EditorVersionRestrictions = editorVersionRestrictionsToModel(ctx, policies.GetEditorVersionRestrictions(), prior.EditorVersionRestrictions, populateUnmanaged, &diags)
	data.RestrictAccountCreationToSCIM = types.BoolValue(policies.GetRestrictAccountCreationToScim())
	data.WebBrowserDisabled = types.BoolValue(policies.GetWebBrowserDisabled())
	data.DisableFromScratch = types.BoolValue(policies.GetDisableFromScratch())
	data.SecurityPolicyID = types.StringValue(policies.GetSecurityPolicyId())
	data.ArchiveEnvironmentsAfter = durationValue(policies.GetArchiveEnvironmentsAfter(), prior.ArchiveEnvironmentsAfter)
	data.AgentPolicy = agentPolicyToModel(ctx, policies.GetAgentPolicy(), prior.AgentPolicy, populateUnmanaged, &diags)
	return diags
}

func durationValue(duration *durationpb.Duration, prior types.String) types.String {
	if duration == nil {
		return types.StringNull()
	}
	actual := duration.AsDuration()
	if !prior.IsNull() && !prior.IsUnknown() {
		parsed, err := time.ParseDuration(prior.ValueString())
		if err == nil && parsed == actual {
			return prior
		}
	}
	return types.StringValue(actual.String())
}

func stringSetValue(ctx context.Context, values []string, prior types.Set, populateUnmanaged bool, diags *diag.Diagnostics) types.Set {
	if !populateUnmanaged && (prior.IsNull() || prior.IsUnknown()) {
		return prior
	}
	if len(values) == 0 && !prior.IsNull() && !prior.IsUnknown() && len(prior.Elements()) == 0 {
		return prior
	}
	result, setDiags := types.SetValueFrom(ctx, types.StringType, values)
	diags.Append(setDiags...)
	return result
}

func stringSliceFromSet(ctx context.Context, set types.Set, diags *diag.Diagnostics) []string {
	if set.IsNull() || set.IsUnknown() {
		return nil
	}
	var values []string
	diags.Append(set.ElementsAs(ctx, &values, false)...)
	sort.Strings(values)
	return values
}

func editorVersionRestrictionsFromModel(ctx context.Context, restrictions []EditorVersionRestrictionModel, diags *diag.Diagnostics) map[string]*v1.EditorVersionPolicy {
	result := make(map[string]*v1.EditorVersionPolicy, len(restrictions))
	for _, restriction := range restrictions {
		versions := stringSliceFromSet(ctx, restriction.AllowedVersions, diags)
		result[restriction.EditorID.ValueString()] = &v1.EditorVersionPolicy{
			AllowedVersions: versions,
		}
	}
	return result
}

func editorVersionRestrictionsToModel(ctx context.Context, restrictions map[string]*v1.EditorVersionPolicy, prior []EditorVersionRestrictionModel, populateUnmanaged bool, diags *diag.Diagnostics) []EditorVersionRestrictionModel {
	if !populateUnmanaged && prior == nil {
		return nil
	}
	if len(restrictions) == 0 && len(prior) == 0 {
		return nil
	}
	editorIDs := make([]string, 0, len(restrictions))
	for editorID := range restrictions {
		editorIDs = append(editorIDs, editorID)
	}
	sort.Strings(editorIDs)

	result := make([]EditorVersionRestrictionModel, 0, len(editorIDs))
	for _, editorID := range editorIDs {
		policy := restrictions[editorID]
		priorVersions := priorEditorVersions(editorID, prior)
		result = append(result, EditorVersionRestrictionModel{
			EditorID:        types.StringValue(editorID),
			AllowedVersions: stringSetValue(ctx, policy.GetAllowedVersions(), priorVersions, populateUnmanaged, diags),
		})
	}
	return result
}

func priorEditorVersions(editorID string, prior []EditorVersionRestrictionModel) types.Set {
	for _, restriction := range prior {
		if restriction.EditorID.ValueString() == editorID {
			return restriction.AllowedVersions
		}
	}
	return types.SetNull(types.StringType)
}

func agentPolicyUpdateFromModel(ctx context.Context, model *AgentPolicyModel, current *v1.AgentPolicy, diags *diag.Diagnostics) *v1.UpdateOrganizationPoliciesRequest_UpdateAgentPolicy {
	if model == nil {
		model = &AgentPolicyModel{}
	}
	if current == nil {
		current = &v1.AgentPolicy{}
	}
	result := &v1.UpdateOrganizationPoliciesRequest_UpdateAgentPolicy{
		AllowedAgentIds:              current.GetAllowedAgentIds(),
		AllowedCodexReasoningEfforts: current.GetAllowedCodexReasoningEfforts(),
		AllowedCodexServiceTiers:     current.GetAllowedCodexServiceTiers(),
	}
	if !model.MCPDisabled.IsNull() && !model.MCPDisabled.IsUnknown() {
		value := model.MCPDisabled.ValueBool()
		result.McpDisabled = &value
	}
	if !model.CommandDenyList.IsNull() && !model.CommandDenyList.IsUnknown() {
		result.CommandDenyList = stringSliceFromSet(ctx, model.CommandDenyList, diags)
	}
	if !model.SCMToolsDisabled.IsNull() && !model.SCMToolsDisabled.IsUnknown() {
		value := model.SCMToolsDisabled.ValueBool()
		result.ScmToolsDisabled = &value
	}
	if !model.SCMToolsAllowedGroupID.IsNull() && !model.SCMToolsAllowedGroupID.IsUnknown() {
		value := model.SCMToolsAllowedGroupID.ValueString()
		result.ScmToolsAllowedGroupId = &value
	}
	if !model.ConversationSharingPolicy.IsNull() && !model.ConversationSharingPolicy.IsUnknown() {
		policy := conversationSharingPolicyFromString(model.ConversationSharingPolicy.ValueString())
		result.ConversationSharingPolicy = &policy
	}
	if !model.MaxSubagentsPerEnvironment.IsNull() && !model.MaxSubagentsPerEnvironment.IsUnknown() {
		value := model.MaxSubagentsPerEnvironment.ValueInt32()
		result.MaxSubagentsPerEnvironment = &value
	}
	if !model.AllowedAgentIDs.IsNull() && !model.AllowedAgentIDs.IsUnknown() {
		result.AllowedAgentIds = stringSliceFromSet(ctx, model.AllowedAgentIDs, diags)
	}
	result.CodexModelPolicy = codexModelPolicyFromMap(ctx, model.CodexModelStates, diags)
	return result
}

func agentPolicyToModel(ctx context.Context, policy *v1.AgentPolicy, prior *AgentPolicyModel, populateUnmanaged bool, diags *diag.Diagnostics) *AgentPolicyModel {
	if policy == nil {
		return nil
	}
	if prior == nil {
		if !populateUnmanaged {
			return nil
		}
		prior = &AgentPolicyModel{}
	}
	return &AgentPolicyModel{
		MCPDisabled:                boolValue(policy.GetMcpDisabled(), prior.MCPDisabled, populateUnmanaged),
		CommandDenyList:            stringSetValue(ctx, policy.GetCommandDenyList(), prior.CommandDenyList, populateUnmanaged, diags),
		SCMToolsDisabled:           boolValue(policy.GetScmToolsDisabled(), prior.SCMToolsDisabled, populateUnmanaged),
		SCMToolsAllowedGroupID:     stringValue(policy.GetScmToolsAllowedGroupId(), prior.SCMToolsAllowedGroupID, populateUnmanaged),
		ConversationSharingPolicy:  conversationSharingPolicyToString(policy.GetConversationSharingPolicy(), prior.ConversationSharingPolicy, populateUnmanaged),
		MaxSubagentsPerEnvironment: int32Value(policy.GetMaxSubagentsPerEnvironment(), prior.MaxSubagentsPerEnvironment, populateUnmanaged),
		AllowedAgentIDs:            stringSetValue(ctx, policy.GetAllowedAgentIds(), prior.AllowedAgentIDs, populateUnmanaged, diags),
		CodexModelStates:           codexModelStatesToMap(ctx, policy.GetCodexModelPolicy(), prior.CodexModelStates, populateUnmanaged, diags),
	}
}

const (
	codexModelStateAllowed  = "allowed"
	codexModelStateDisabled = "disabled"
)

func codexModelPolicyFromMap(ctx context.Context, value types.Map, diags *diag.Diagnostics) *v1.CodexModelPolicy {
	if value.IsNull() || value.IsUnknown() {
		return nil
	}

	modelStates := make(map[string]string, len(value.Elements()))
	diags.Append(value.ElementsAs(ctx, &modelStates, false)...)
	if diags.HasError() {
		return nil
	}

	result := &v1.CodexModelPolicy{
		ModelStates: make(map[string]v1.CodexModelPolicyState, len(modelStates)),
	}
	for model, state := range modelStates {
		entryPath := path.Root("agent_policy").AtName("codex_model_states").AtMapKey(model)
		if !validCodexModelName(model) {
			diags.AddAttributeError(entryPath, "Invalid Codex Model", fmt.Sprintf("%q is not a supported CodexOpenAIModel enum name.", model))
			continue
		}
		switch state {
		case codexModelStateAllowed:
			result.ModelStates[model] = v1.CodexModelPolicyState_CODEX_MODEL_POLICY_STATE_ALLOWED
		case codexModelStateDisabled:
			result.ModelStates[model] = v1.CodexModelPolicyState_CODEX_MODEL_POLICY_STATE_DISABLED
		default:
			diags.AddAttributeError(
				entryPath,
				"Invalid Codex Model State",
				"Supported values are \"allowed\" and \"disabled\".",
			)
		}
	}
	return result
}

func codexModelStatesToMap(ctx context.Context, policy *v1.CodexModelPolicy, prior types.Map, populateUnmanaged bool, diags *diag.Diagnostics) types.Map {
	if !populateUnmanaged && (prior.IsNull() || prior.IsUnknown()) {
		return prior
	}

	modelStates := make(map[string]string)
	if policy != nil {
		for model, state := range policy.GetModelStates() {
			entryPath := path.Root("agent_policy").AtName("codex_model_states").AtMapKey(model)
			if !validCodexModelName(model) {
				diags.AddAttributeError(entryPath, "Unknown Codex Model", fmt.Sprintf("The Ona API returned unsupported Codex model key %q.", model))
				continue
			}
			switch state {
			case v1.CodexModelPolicyState_CODEX_MODEL_POLICY_STATE_UNSPECIFIED,
				v1.CodexModelPolicyState_CODEX_MODEL_POLICY_STATE_ALLOWED:
				modelStates[model] = codexModelStateAllowed
			case v1.CodexModelPolicyState_CODEX_MODEL_POLICY_STATE_DISABLED:
				modelStates[model] = codexModelStateDisabled
			default:
				diags.AddAttributeError(entryPath, "Unknown Codex Model State", fmt.Sprintf("The Ona API returned unsupported Codex model policy state %d.", state))
			}
		}
	}

	if !prior.IsNull() && !prior.IsUnknown() {
		var priorStates map[string]string
		diags.Append(prior.ElementsAs(ctx, &priorStates, false)...)
		if !diags.HasError() {
			for model, state := range priorStates {
				if state == codexModelStateAllowed {
					if _, present := modelStates[model]; !present {
						modelStates[model] = codexModelStateAllowed
					}
				}
			}
		}
	}

	result, valueDiags := types.MapValueFrom(ctx, types.StringType, modelStates)
	diags.Append(valueDiags...)
	return result
}

func validateCodexModelStates(attrPath path.Path, value types.Map, diags *diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return
	}
	for model, rawState := range value.Elements() {
		entryPath := attrPath.AtMapKey(model)
		if !validCodexModelName(model) {
			diags.AddAttributeError(entryPath, "Invalid Codex Model", fmt.Sprintf("%q is not a supported CodexOpenAIModel enum name.", model))
		}
		state, ok := rawState.(types.String)
		if !ok || state.IsNull() {
			diags.AddAttributeError(entryPath, "Invalid Codex Model State", "Supported values are \"allowed\" and \"disabled\".")
			continue
		}
		if state.IsUnknown() {
			continue
		}
		switch state.ValueString() {
		case codexModelStateAllowed, codexModelStateDisabled:
		default:
			diags.AddAttributeError(entryPath, "Invalid Codex Model State", "Supported values are \"allowed\" and \"disabled\".")
		}
	}
}

func validCodexModelName(model string) bool {
	value, ok := v1.CodexOpenAIModel_value[model]
	return ok && v1.CodexOpenAIModel(value) != v1.CodexOpenAIModel_CODEX_OPEN_AI_MODEL_UNSPECIFIED
}

func cloneCodexModelPolicy(policy *v1.CodexModelPolicy) *v1.CodexModelPolicy {
	if policy == nil {
		return nil
	}
	result := &v1.CodexModelPolicy{
		ModelStates: make(map[string]v1.CodexModelPolicyState, len(policy.GetModelStates())),
	}
	for model, state := range policy.GetModelStates() {
		result.ModelStates[model] = state
	}
	return result
}

func codexModelPolicyForRestore(policy *v1.CodexModelPolicy) *v1.CodexModelPolicy {
	if policy == nil {
		return &v1.CodexModelPolicy{}
	}
	return cloneCodexModelPolicy(policy)
}

func boolValue(value bool, prior types.Bool, populateUnmanaged bool) types.Bool {
	if !populateUnmanaged && (prior.IsNull() || prior.IsUnknown()) {
		return prior
	}
	return types.BoolValue(value)
}

func stringValue(value string, prior types.String, populateUnmanaged bool) types.String {
	if !populateUnmanaged && (prior.IsNull() || prior.IsUnknown()) {
		return prior
	}
	return types.StringValue(value)
}

func int32Value(value int32, prior types.Int32, populateUnmanaged bool) types.Int32 {
	if !populateUnmanaged && (prior.IsNull() || prior.IsUnknown()) {
		return prior
	}
	return types.Int32Value(value)
}

func conversationSharingPolicyFromString(value string) v1.ConversationSharingPolicy {
	switch value {
	case conversationSharingDisabled:
		return v1.ConversationSharingPolicy_CONVERSATION_SHARING_POLICY_DISABLED
	case conversationSharingOrganization:
		return v1.ConversationSharingPolicy_CONVERSATION_SHARING_POLICY_ORGANIZATION
	default:
		return v1.ConversationSharingPolicy_CONVERSATION_SHARING_POLICY_UNSPECIFIED
	}
}

func conversationSharingPolicyToString(value v1.ConversationSharingPolicy, prior types.String, populateUnmanaged bool) types.String {
	if !populateUnmanaged && (prior.IsNull() || prior.IsUnknown()) {
		return prior
	}
	switch value {
	case v1.ConversationSharingPolicy_CONVERSATION_SHARING_POLICY_DISABLED:
		return types.StringValue(conversationSharingDisabled)
	case v1.ConversationSharingPolicy_CONVERSATION_SHARING_POLICY_ORGANIZATION:
		return types.StringValue(conversationSharingOrganization)
	default:
		return types.StringNull()
	}
}

func validateAgentPolicy(policy *AgentPolicyModel, diags *diag.Diagnostics) {
	if policy == nil {
		return
	}
	if !policy.ConversationSharingPolicy.IsNull() && !policy.ConversationSharingPolicy.IsUnknown() {
		validateConversationSharingPolicy(path.Root("agent_policy").AtName("conversation_sharing_policy"), policy.ConversationSharingPolicy.ValueString(), diags)
	}
	if !policy.MaxSubagentsPerEnvironment.IsNull() && !policy.MaxSubagentsPerEnvironment.IsUnknown() {
		validateMaxSubagents(path.Root("agent_policy").AtName("max_subagents_per_environment"), policy.MaxSubagentsPerEnvironment.ValueInt32(), diags)
	}
	validateCodexModelStates(path.Root("agent_policy").AtName("codex_model_states"), policy.CodexModelStates, diags)
}

func validateConversationSharingPolicy(attrPath path.Path, value string, diags *diag.Diagnostics) {
	switch value {
	case "", conversationSharingDisabled, conversationSharingOrganization:
	default:
		diags.AddAttributeError(attrPath, "Invalid Conversation Sharing Policy", "Supported values are \"disabled\" and \"organization\".")
	}
}

func validateMaxSubagents(attrPath path.Path, value int32, diags *diag.Diagnostics) {
	if value < 0 || value > 10 {
		diags.AddAttributeError(attrPath, "Invalid Max Subagents", "max_subagents_per_environment must be between 0 and 10.")
	}
}
