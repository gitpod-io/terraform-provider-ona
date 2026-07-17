// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package organization

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/durationpb"
)

var _ resource.Resource = &PoliciesResource{}
var _ resource.ResourceWithConfigure = &PoliciesResource{}
var _ resource.ResourceWithIdentity = &PoliciesResource{}
var _ resource.ResourceWithImportState = &PoliciesResource{}
var _ resource.ResourceWithValidateConfig = &PoliciesResource{}

func NewPoliciesResource() resource.Resource {
	return &PoliciesResource{}
}

type PoliciesResource struct {
	clientHolder
}

type PoliciesModel struct {
	ID                                types.String                    `tfsdk:"id"`
	MaximumEnvironmentTimeout         types.String                    `tfsdk:"maximum_environment_timeout"`
	MembersRequireProjects            types.Bool                      `tfsdk:"members_require_projects"`
	MembersCreateProjects             types.Bool                      `tfsdk:"members_create_projects"`
	AllowedEditorIDs                  types.Set                       `tfsdk:"allowed_editor_ids"`
	DefaultEditorID                   types.String                    `tfsdk:"default_editor_id"`
	AllowLocalRunners                 types.Bool                      `tfsdk:"allow_local_runners"`
	MaximumRunningEnvironmentsPerUser types.Int64                     `tfsdk:"maximum_running_environments_per_user"`
	MaximumEnvironmentsPerUser        types.Int64                     `tfsdk:"maximum_environments_per_user"`
	DefaultEnvironmentImage           types.String                    `tfsdk:"default_environment_image"`
	PortSharingDisabled               types.Bool                      `tfsdk:"port_sharing_disabled"`
	DeleteArchivedEnvironmentsAfter   types.String                    `tfsdk:"delete_archived_environments_after"`
	MaximumEnvironmentLifetime        types.String                    `tfsdk:"maximum_environment_lifetime"`
	RequireCustomDomainAccess         types.Bool                      `tfsdk:"require_custom_domain_access"`
	EditorVersionRestrictions         []EditorVersionRestrictionModel `tfsdk:"editor_version_restriction"`
	RestrictAccountCreationToSCIM     types.Bool                      `tfsdk:"restrict_account_creation_to_scim"`
	WebBrowserDisabled                types.Bool                      `tfsdk:"web_browser_disabled"`
	DisableFromScratch                types.Bool                      `tfsdk:"disable_from_scratch"`
	SecurityPolicyID                  types.String                    `tfsdk:"security_policy_id"`
	ArchiveEnvironmentsAfter          types.String                    `tfsdk:"archive_environments_after"`
	AgentPolicy                       *AgentPolicyModel               `tfsdk:"agent_policy"`
}

type EditorVersionRestrictionModel struct {
	EditorID        types.String `tfsdk:"editor_id"`
	AllowedVersions types.Set    `tfsdk:"allowed_versions"`
}

type AgentPolicyModel struct {
	MCPDisabled                types.Bool   `tfsdk:"mcp_disabled"`
	CommandDenyList            types.Set    `tfsdk:"command_deny_list"`
	SCMToolsDisabled           types.Bool   `tfsdk:"scm_tools_disabled"`
	SCMToolsAllowedGroupID     types.String `tfsdk:"scm_tools_allowed_group_id"`
	ConversationSharingPolicy  types.String `tfsdk:"conversation_sharing_policy"`
	MaxSubagentsPerEnvironment types.Int32  `tfsdk:"max_subagents_per_environment"`
	AllowedAgentIDs            types.Set    `tfsdk:"allowed_agent_ids"`
}

const (
	conversationSharingDisabled     = "disabled"
	conversationSharingOrganization = "organization"
	policiesBaselinePrivateKey      = "server_defined_policy_baseline"
)

func (r *PoliciesResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_organization_policies"
}

func (r *PoliciesResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		MarkdownDescription: "Singleton Ona organization policy settings for the organization associated with the authenticated provider token. This resource updates the remote policy object in place. Destroying it restores the server-defined policy configuration that existed before Terraform began managing it.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Terraform resource ID. This is the authenticated organization ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"maximum_environment_timeout": durationAttribute("Maximum timeout allowed for environments. `0s` means no limit; non-zero values must be at least `30m`."),
			"members_require_projects": resourceschema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether non-admin users can only create environments from projects. Configure together with `members_create_projects`.",
			},
			"members_create_projects": resourceschema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether organization members can create projects. Configure together with `members_require_projects`.",
			},
			"allowed_editor_ids": resourceschema.SetAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Editor IDs allowed in the organization. The current Ona API cannot clear this field through Terraform; omit it to leave it unmanaged.",
			},
			"default_editor_id": stringOptionalComputedAttribute("Default editor ID."),
			"allow_local_runners": resourceschema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether local runners are allowed. The Ona API rejects enabling local runners through organization policies.",
			},
			"maximum_running_environments_per_user": resourceschema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Maximum simultaneously running environments per user. Omit to leave the remote setting unmanaged.",
			},
			"maximum_environments_per_user": resourceschema.Int64Attribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Maximum total environments per user. Omit to leave the remote setting unmanaged.",
			},
			"default_environment_image": stringOptionalComputedAttribute("Default container image when no repository image is defined."),
			"port_sharing_disabled": resourceschema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether user-initiated port sharing is disabled. Omit to leave the remote setting unmanaged.",
			},
			"delete_archived_environments_after": durationAttribute("How long archived environments are kept before automatic deletion. `0s` disables automatic deletion; maximum is 672h."),
			"maximum_environment_lifetime":       durationAttribute("How long environments may be reused. `0s` means no maximum; maximum is 4320h."),
			"require_custom_domain_access": resourceschema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether users must access via a configured custom domain. Omit to leave the remote setting unmanaged.",
			},
			"editor_version_restriction": resourceschema.ListNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Editor version restrictions keyed by editor ID. Omit to leave editor version restrictions unmanaged.",
				NestedObject: resourceschema.NestedAttributeObject{
					Attributes: map[string]resourceschema.Attribute{
						"editor_id": resourceschema.StringAttribute{
							Required:            true,
							MarkdownDescription: "Editor ID.",
						},
						"allowed_versions": resourceschema.SetAttribute{
							Optional:            true,
							ElementType:         types.StringType,
							MarkdownDescription: "Allowed editor versions. Empty means latest.",
						},
					},
				},
			},
			"restrict_account_creation_to_scim": resourceschema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether account creation is restricted to SCIM-provisioned users. Omit to leave the remote setting unmanaged.",
			},
			"web_browser_disabled": resourceschema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether users can open the built-in web browser from environment pages. Omit to leave the remote setting unmanaged.",
			},
			"disable_from_scratch": resourceschema.BoolAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Whether non-admin users can create blank environments without a Git or URL initializer. Omit to leave the remote setting unmanaged.",
			},
			"security_policy_id":         stringOptionalComputedAttribute("Default security policy ID assigned to newly created environments. Set an empty string to clear."),
			"archive_environments_after": durationAttribute("How long stopped environments remain inactive before archival. Must be a whole number of days between 24h and 720h."),
			"agent_policy": resourceschema.SingleNestedAttribute{
				Optional:            true,
				MarkdownDescription: "Agent-specific organization policy settings. Omit the block to leave agent policy fields unmanaged.",
				Attributes: map[string]resourceschema.Attribute{
					"mcp_disabled": resourceschema.BoolAttribute{
						Optional:            true,
						MarkdownDescription: "Whether MCP is disabled for agents.",
					},
					"command_deny_list": resourceschema.SetAttribute{
						Optional:            true,
						ElementType:         types.StringType,
						MarkdownDescription: "Commands agents are not allowed to execute. Omit to leave the deny list unmanaged.",
					},
					"scm_tools_disabled": resourceschema.BoolAttribute{
						Optional:            true,
						MarkdownDescription: "Whether SCM tools are disabled for agents.",
					},
					"scm_tools_allowed_group_id": resourceschema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Group ID allowed to use SCM tools. Empty means unrestricted.",
					},
					"conversation_sharing_policy": resourceschema.StringAttribute{
						Optional:            true,
						MarkdownDescription: "Conversation sharing policy. Supported values are `disabled` and `organization`.",
					},
					"max_subagents_per_environment": resourceschema.Int32Attribute{
						Optional:            true,
						MarkdownDescription: "Maximum non-terminal sub-agents per environment. Valid range is 0-10.",
					},
					"allowed_agent_ids": resourceschema.SetAttribute{
						Optional:            true,
						ElementType:         types.StringType,
						MarkdownDescription: "Agent IDs users may select. Empty means all agents are allowed.",
					},
				},
			},
		},
	}
}

func stringOptionalComputedAttribute(description string) resourceschema.StringAttribute {
	return resourceschema.StringAttribute{
		Optional:            true,
		Computed:            true,
		MarkdownDescription: description,
	}
}

func durationAttribute(description string) resourceschema.StringAttribute {
	return resourceschema.StringAttribute{
		Optional:            true,
		Computed:            true,
		MarkdownDescription: description + " Use Go duration strings such as `30m`, `24h`, or `0s`.",
	}
}

func (r *PoliciesResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.configure(req, resp)
}

func (r *PoliciesResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	resp.Diagnostics.Append(validatePoliciesConfig(ctx, req.Config)...)
}

func (r *PoliciesResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data PoliciesModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.requireClient(&resp.Diagnostics, "creating", "ona_organization_policies") {
		return
	}
	organizationID, err := r.authenticatedOrganizationID(ctx)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Authenticated Ona Organization", "resolving the authenticated Ona organization", err)
		return
	}

	current, err := r.getPolicies(ctx, organizationID)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Organization Policies", "reading current Ona organization policies before update", err)
		return
	}
	resp.Diagnostics.Append(setPoliciesBaseline(ctx, resp.Private, current)...)
	if resp.Diagnostics.HasError() {
		return
	}
	updateReq, diags := updatePoliciesRequestFromConfig(ctx, organizationID, data, req.Config, current)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.OrganizationService().UpdateOrganizationPolicies(ctx, connect.NewRequest(updateReq)); err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Update Ona Organization Policies", "updating Ona organization policies", err)
		return
	}
	data.ID = types.StringValue(organizationID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	policies, err := r.getPolicies(ctx, organizationID)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Updated Ona Organization Policies", "reading updated Ona organization policies", err)
		return
	}
	planned := data
	resp.Diagnostics.Append(populatePoliciesModel(ctx, &data, policies, planned, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, PoliciesIdentityModel{
		OrganizationID: types.StringValue(organizationID),
	})...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PoliciesResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data PoliciesModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.requireClient(&resp.Diagnostics, "reading", "ona_organization_policies") {
		return
	}
	organizationID, err := r.authenticatedOrganizationID(ctx)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Authenticated Ona Organization", "resolving the authenticated Ona organization", err)
		return
	}
	if !guardStateOrganizationID(&resp.Diagnostics, data.ID, organizationID, "ona_organization_policies") {
		return
	}

	policies, err := r.getPolicies(ctx, organizationID)
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Organization Policies", "reading Ona organization policies", err)
		return
	}
	resp.Diagnostics.Append(ensurePoliciesBaseline(ctx, req.Private, resp.Private, policies)...)
	if resp.Diagnostics.HasError() {
		return
	}
	prior := data
	data = PoliciesModel{}
	resp.Diagnostics.Append(populatePoliciesModel(ctx, &data, policies, prior, shouldPopulateUnmanagedPolicyFields(prior))...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, PoliciesIdentityModel{
		OrganizationID: types.StringValue(organizationID),
	})...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PoliciesResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data PoliciesModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.requireClient(&resp.Diagnostics, "updating", "ona_organization_policies") {
		return
	}
	organizationID, err := r.authenticatedOrganizationID(ctx)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Authenticated Ona Organization", "resolving the authenticated Ona organization", err)
		return
	}
	if !guardStateOrganizationID(&resp.Diagnostics, data.ID, organizationID, "ona_organization_policies") {
		return
	}

	current, err := r.getPolicies(ctx, organizationID)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Organization Policies", "reading current Ona organization policies before update", err)
		return
	}
	resp.Diagnostics.Append(ensurePoliciesBaseline(ctx, req.Private, resp.Private, current)...)
	if resp.Diagnostics.HasError() {
		return
	}
	updateReq, diags := updatePoliciesRequestFromConfig(ctx, organizationID, data, req.Config, current)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.OrganizationService().UpdateOrganizationPolicies(ctx, connect.NewRequest(updateReq)); err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Update Ona Organization Policies", "updating Ona organization policies", err)
		return
	}

	policies, err := r.getPolicies(ctx, organizationID)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Updated Ona Organization Policies", "reading updated Ona organization policies", err)
		return
	}
	planned := data
	resp.Diagnostics.Append(populatePoliciesModel(ctx, &data, policies, planned, false)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, PoliciesIdentityModel{
		OrganizationID: types.StringValue(organizationID),
	})...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func shouldPopulateUnmanagedPolicyFields(prior PoliciesModel) bool {
	return !prior.ID.IsNull() &&
		prior.MaximumEnvironmentTimeout.IsNull() &&
		prior.MembersRequireProjects.IsNull() &&
		prior.DefaultEditorID.IsNull()
}

func (r *PoliciesResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data PoliciesModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "deleting", "ona_organization_policies") {
		return
	}
	organizationID, err := r.authenticatedOrganizationID(ctx)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Authenticated Ona Organization", "resolving the authenticated Ona organization", err)
		return
	}
	if !guardStateOrganizationID(&resp.Diagnostics, data.ID, organizationID, "ona_organization_policies") {
		return
	}
	baseline, diags := policiesBaseline(ctx, req.Private)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	current, err := r.getPolicies(ctx, organizationID)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Organization Policies", "reading current Ona organization policies before restoring defaults", err)
		return
	}
	if _, err := r.client.OrganizationService().UpdateOrganizationPolicies(ctx, connect.NewRequest(restorePoliciesRequest(organizationID, baseline, current))); err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Restore Ona Organization Policies", "restoring the server-defined Ona organization policy configuration", err)
		return
	}
	resp.State.RemoveResource(ctx)
}

func (r *PoliciesResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !r.requireClient(&resp.Diagnostics, "importing", "ona_organization_policies") {
		return
	}
	organizationID := req.ID
	if organizationID == "" {
		var identity PoliciesIdentityModel
		resp.Diagnostics.Append(req.Identity.Get(ctx, &identity)...)
		if resp.Diagnostics.HasError() {
			return
		}
		organizationID = identity.OrganizationID.ValueString()
	} else {
		authenticatedOrganizationID, err := r.authenticatedOrganizationID(ctx)
		if err != nil {
			providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Authenticated Ona Organization", "resolving the authenticated Ona organization", err)
			return
		}
		if organizationID == "current" {
			organizationID = authenticatedOrganizationID
		} else if organizationID != authenticatedOrganizationID {
			resp.Diagnostics.AddError("Invalid Ona Organization Policies Import ID", fmt.Sprintf("Import ona_organization_policies with \"current\" or the authenticated organization ID %q.", authenticatedOrganizationID))
			return
		}
	}
	baseline, err := r.getPolicies(ctx, organizationID)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Organization Policies", "reading Ona organization policies during import", err)
		return
	}
	resp.Diagnostics.Append(setPoliciesBaseline(ctx, resp.Private, baseline)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), organizationID)...)
	resp.Diagnostics.Append(resp.Identity.Set(ctx, PoliciesIdentityModel{OrganizationID: types.StringValue(organizationID)})...)
}

func (r *PoliciesResource) getPolicies(ctx context.Context, organizationID string) (*v1.OrganizationPolicies, error) {
	result, err := r.client.OrganizationService().GetOrganizationPolicies(ctx, connect.NewRequest(&v1.GetOrganizationPoliciesRequest{
		OrganizationId: organizationID,
	}))
	if err != nil {
		return nil, fmt.Errorf("get organization policies: %w", err)
	}
	return result.Msg.GetPolicies(), nil
}

func setPoliciesBaseline(ctx context.Context, state privateState, policies *v1.OrganizationPolicies) diag.Diagnostics {
	var diags diag.Diagnostics
	if state == nil {
		diags.AddError("Unable to Store Organization Policy Baseline", "Terraform did not provide private state storage for ona_organization_policies.")
		return diags
	}
	if policies == nil {
		diags.AddError("Unable to Store Organization Policy Baseline", "The Ona API returned an empty organization policy object.")
		return diags
	}
	data, err := protojson.Marshal(managedPoliciesSnapshot(policies))
	if err != nil {
		diags.AddError("Unable to Store Organization Policy Baseline", fmt.Sprintf("Could not encode the server-defined organization policy configuration: %s.", err))
		return diags
	}
	diags.Append(state.SetKey(ctx, policiesBaselinePrivateKey, data)...)
	return diags
}

func ensurePoliciesBaseline(ctx context.Context, prior privateState, next privateState, policies *v1.OrganizationPolicies) diag.Diagnostics {
	var diags diag.Diagnostics
	if prior != nil {
		data, stateDiags := prior.GetKey(ctx, policiesBaselinePrivateKey)
		diags.Append(stateDiags...)
		if diags.HasError() {
			return diags
		}
		if len(data) > 0 {
			if next == nil {
				diags.AddError("Unable to Store Organization Policy Baseline", "Terraform did not provide private state storage for ona_organization_policies.")
				return diags
			}
			diags.Append(next.SetKey(ctx, policiesBaselinePrivateKey, data)...)
			return diags
		}
	}
	return setPoliciesBaseline(ctx, next, policies)
}

func policiesBaseline(ctx context.Context, state privateState) (*v1.OrganizationPolicies, diag.Diagnostics) {
	var diags diag.Diagnostics
	if state == nil {
		diags.AddError("Unable to Restore Organization Policy Baseline", "Terraform did not provide private state storage for ona_organization_policies.")
		return nil, diags
	}
	data, stateDiags := state.GetKey(ctx, policiesBaselinePrivateKey)
	diags.Append(stateDiags...)
	if diags.HasError() {
		return nil, diags
	}
	if len(data) == 0 {
		diags.AddError("Unable to Restore Organization Policy Baseline", "The server-defined organization policy configuration is missing from private state. Refresh or re-import the resource before deleting it.")
		return nil, diags
	}
	var policies v1.OrganizationPolicies
	if err := protojson.Unmarshal(data, &policies); err != nil {
		diags.AddError("Unable to Restore Organization Policy Baseline", fmt.Sprintf("Could not decode the server-defined organization policy configuration: %s.", err))
		return nil, diags
	}
	return &policies, diags
}

func managedPoliciesSnapshot(policies *v1.OrganizationPolicies) *v1.OrganizationPolicies {
	var agentPolicy *v1.AgentPolicy
	if policy := policies.GetAgentPolicy(); policy != nil {
		agentPolicy = &v1.AgentPolicy{
			McpDisabled:                policy.GetMcpDisabled(),
			CommandDenyList:            append([]string(nil), policy.GetCommandDenyList()...),
			ScmToolsDisabled:           policy.GetScmToolsDisabled(),
			ScmToolsAllowedGroupId:     policy.GetScmToolsAllowedGroupId(),
			ConversationSharingPolicy:  policy.GetConversationSharingPolicy(),
			MaxSubagentsPerEnvironment: policy.GetMaxSubagentsPerEnvironment(),
			AllowedAgentIds:            append([]string(nil), policy.GetAllowedAgentIds()...),
		}
	}
	return &v1.OrganizationPolicies{
		OrganizationId:                    policies.GetOrganizationId(),
		MaximumEnvironmentTimeout:         policies.GetMaximumEnvironmentTimeout(),
		MembersRequireProjects:            policies.GetMembersRequireProjects(),
		MembersCreateProjects:             policies.GetMembersCreateProjects(),
		AllowedEditorIds:                  append([]string(nil), policies.GetAllowedEditorIds()...),
		DefaultEditorId:                   policies.GetDefaultEditorId(),
		AllowLocalRunners:                 policies.GetAllowLocalRunners(),
		MaximumRunningEnvironmentsPerUser: policies.GetMaximumRunningEnvironmentsPerUser(),
		MaximumEnvironmentsPerUser:        policies.GetMaximumEnvironmentsPerUser(),
		DefaultEnvironmentImage:           policies.GetDefaultEnvironmentImage(),
		PortSharingDisabled:               policies.GetPortSharingDisabled(),
		DeleteArchivedEnvironmentsAfter:   policies.GetDeleteArchivedEnvironmentsAfter(),
		AgentPolicy:                       agentPolicy,
		MaximumEnvironmentLifetime:        policies.GetMaximumEnvironmentLifetime(),
		RequireCustomDomainAccess:         policies.GetRequireCustomDomainAccess(),
		EditorVersionRestrictions:         policies.GetEditorVersionRestrictions(),
		RestrictAccountCreationToScim:     policies.GetRestrictAccountCreationToScim(),
		WebBrowserDisabled:                policies.GetWebBrowserDisabled(),
		DisableFromScratch:                policies.GetDisableFromScratch(),
		SecurityPolicyId:                  policies.GetSecurityPolicyId(),
		ArchiveEnvironmentsAfter:          policies.GetArchiveEnvironmentsAfter(),
	}
}

func restorePoliciesRequest(organizationID string, baseline *v1.OrganizationPolicies, current *v1.OrganizationPolicies) *v1.UpdateOrganizationPoliciesRequest {
	baselineAgentPolicy := baseline.GetAgentPolicy()
	if baselineAgentPolicy == nil {
		baselineAgentPolicy = &v1.AgentPolicy{}
	}
	currentAgentPolicy := current.GetAgentPolicy()
	if currentAgentPolicy == nil {
		currentAgentPolicy = &v1.AgentPolicy{}
	}
	return &v1.UpdateOrganizationPoliciesRequest{
		OrganizationId:                    organizationID,
		MaximumEnvironmentTimeout:         baseline.GetMaximumEnvironmentTimeout(),
		MembersRequireProjects:            ptr(baseline.GetMembersRequireProjects()),
		MembersCreateProjects:             ptr(baseline.GetMembersCreateProjects()),
		AllowedEditorIds:                  append([]string(nil), baseline.GetAllowedEditorIds()...),
		DefaultEditorId:                   ptr(baseline.GetDefaultEditorId()),
		AllowLocalRunners:                 ptr(baseline.GetAllowLocalRunners()),
		MaximumRunningEnvironmentsPerUser: ptr(baseline.GetMaximumRunningEnvironmentsPerUser()),
		MaximumEnvironmentsPerUser:        ptr(baseline.GetMaximumEnvironmentsPerUser()),
		DefaultEnvironmentImage:           ptr(baseline.GetDefaultEnvironmentImage()),
		PortSharingDisabled:               ptr(baseline.GetPortSharingDisabled()),
		DeleteArchivedEnvironmentsAfter:   baseline.GetDeleteArchivedEnvironmentsAfter(),
		MaximumEnvironmentLifetime:        baseline.GetMaximumEnvironmentLifetime(),
		RequireCustomDomainAccess:         ptr(baseline.GetRequireCustomDomainAccess()),
		EditorVersionRestrictions:         baseline.GetEditorVersionRestrictions(),
		RestrictAccountCreationToScim:     ptr(baseline.GetRestrictAccountCreationToScim()),
		WebBrowserDisabled:                ptr(baseline.GetWebBrowserDisabled()),
		DisableFromScratch:                ptr(baseline.GetDisableFromScratch()),
		SecurityPolicyId:                  ptr(baseline.GetSecurityPolicyId()),
		ArchiveEnvironmentsAfter:          baseline.GetArchiveEnvironmentsAfter(),
		AgentPolicy: &v1.UpdateOrganizationPoliciesRequest_UpdateAgentPolicy{
			McpDisabled:                  ptr(baselineAgentPolicy.GetMcpDisabled()),
			CommandDenyList:              append([]string(nil), baselineAgentPolicy.GetCommandDenyList()...),
			ScmToolsDisabled:             ptr(baselineAgentPolicy.GetScmToolsDisabled()),
			ScmToolsAllowedGroupId:       ptr(baselineAgentPolicy.GetScmToolsAllowedGroupId()),
			ConversationSharingPolicy:    ptr(baselineAgentPolicy.GetConversationSharingPolicy()),
			MaxSubagentsPerEnvironment:   ptr(baselineAgentPolicy.GetMaxSubagentsPerEnvironment()),
			AllowedAgentIds:              append([]string(nil), baselineAgentPolicy.GetAllowedAgentIds()...),
			AllowedCodexModels:           append([]v1.CodexOpenAIModel(nil), currentAgentPolicy.GetAllowedCodexModels()...), //nolint:staticcheck // Existing Terraform schema still maps the legacy allowlist.
			AllowedCodexReasoningEfforts: append([]v1.CodexReasoningEffort(nil), currentAgentPolicy.GetAllowedCodexReasoningEfforts()...),
			AllowedCodexServiceTiers:     append([]v1.CodexServiceTier(nil), currentAgentPolicy.GetAllowedCodexServiceTiers()...),
		},
	}
}

func validatePoliciesConfig(ctx context.Context, cfg tfsdk.Config) diag.Diagnostics {
	var diags diag.Diagnostics
	diags.Append(validateDuration(ctx, cfg, path.Root("maximum_environment_timeout"), 30*time.Minute, 0)...)
	diags.Append(validateDuration(ctx, cfg, path.Root("delete_archived_environments_after"), 0, 28*24*time.Hour)...)
	diags.Append(validateDuration(ctx, cfg, path.Root("maximum_environment_lifetime"), 0, 180*24*time.Hour)...)
	validateArchiveEnvironmentsAfter(ctx, cfg, &diags)
	validateMembersProjectPair(ctx, cfg, &diags)
	validateAllowedEditorIDs(ctx, cfg, &diags)
	validateAgentPolicyConfig(ctx, cfg, &diags)
	return diags
}

func validateDuration(ctx context.Context, cfg tfsdk.Config, attrPath path.Path, minNonZero time.Duration, maxDuration time.Duration) diag.Diagnostics {
	var diags diag.Diagnostics
	var value types.String
	diags.Append(cfg.GetAttribute(ctx, attrPath, &value)...)
	if diags.HasError() || value.IsNull() || value.IsUnknown() {
		return diags
	}
	duration, err := time.ParseDuration(value.ValueString())
	if err != nil {
		diags.AddAttributeError(attrPath, "Invalid Duration", "Use a Go duration string such as \"30m\", \"24h\", or \"0s\".")
		return diags
	}
	if duration < 0 {
		diags.AddAttributeError(attrPath, "Invalid Duration", "Duration must not be negative.")
		return diags
	}
	if duration != 0 && minNonZero > 0 && duration < minNonZero {
		diags.AddAttributeError(attrPath, "Invalid Duration", fmt.Sprintf("Duration must be 0s or at least %s.", minNonZero))
	}
	if maxDuration > 0 && duration > maxDuration {
		diags.AddAttributeError(attrPath, "Invalid Duration", fmt.Sprintf("Duration must be at most %s.", maxDuration))
	}
	return diags
}

func validateArchiveEnvironmentsAfter(ctx context.Context, cfg tfsdk.Config, diags *diag.Diagnostics) {
	var value types.String
	diags.Append(cfg.GetAttribute(ctx, path.Root("archive_environments_after"), &value)...)
	if diags.HasError() || value.IsNull() || value.IsUnknown() {
		return
	}
	duration, err := time.ParseDuration(value.ValueString())
	if err != nil {
		diags.AddAttributeError(path.Root("archive_environments_after"), "Invalid Duration", "Use a Go duration string such as \"24h\".")
		return
	}
	if duration < 24*time.Hour || duration > 30*24*time.Hour || duration%(24*time.Hour) != 0 {
		diags.AddAttributeError(path.Root("archive_environments_after"), "Invalid Archive Duration", "archive_environments_after must be a whole number of days between 24h and 720h.")
	}
}

func validateMembersProjectPair(ctx context.Context, cfg tfsdk.Config, diags *diag.Diagnostics) {
	var requireProjects types.Bool
	var createProjects types.Bool
	diags.Append(cfg.GetAttribute(ctx, path.Root("members_require_projects"), &requireProjects)...)
	diags.Append(cfg.GetAttribute(ctx, path.Root("members_create_projects"), &createProjects)...)
	if diags.HasError() || requireProjects.IsUnknown() || createProjects.IsUnknown() {
		return
	}
	if requireProjects.IsNull() != createProjects.IsNull() {
		diags.AddAttributeError(path.Root("members_require_projects"), "Invalid Project Member Policy", "members_require_projects and members_create_projects must be configured together.")
		return
	}
	if !requireProjects.IsNull() && requireProjects.ValueBool() == createProjects.ValueBool() {
		diags.AddAttributeError(path.Root("members_create_projects"), "Invalid Project Member Policy", "members_require_projects and members_create_projects must have opposite values.")
	}
}

func validateAllowedEditorIDs(ctx context.Context, cfg tfsdk.Config, diags *diag.Diagnostics) {
	var value types.Set
	diags.Append(cfg.GetAttribute(ctx, path.Root("allowed_editor_ids"), &value)...)
	if diags.HasError() || value.IsNull() || value.IsUnknown() {
		return
	}
	if len(value.Elements()) == 0 {
		diags.AddAttributeError(path.Root("allowed_editor_ids"), "Cannot Clear Allowed Editor IDs", "The Ona API currently ignores an empty allowed_editor_ids update. Omit this attribute to leave it unmanaged.")
	}
}

func validateAgentPolicyConfig(ctx context.Context, cfg tfsdk.Config, diags *diag.Diagnostics) {
	var conversationSharingPolicy types.String
	conversationSharingPath := path.Root("agent_policy").AtName("conversation_sharing_policy")
	diags.Append(cfg.GetAttribute(ctx, conversationSharingPath, &conversationSharingPolicy)...)
	if !diags.HasError() && !conversationSharingPolicy.IsNull() && !conversationSharingPolicy.IsUnknown() {
		validateConversationSharingPolicy(conversationSharingPath, conversationSharingPolicy.ValueString(), diags)
	}

	var maxSubagents types.Int32
	maxSubagentsPath := path.Root("agent_policy").AtName("max_subagents_per_environment")
	diags.Append(cfg.GetAttribute(ctx, maxSubagentsPath, &maxSubagents)...)
	if !diags.HasError() && !maxSubagents.IsNull() && !maxSubagents.IsUnknown() {
		validateMaxSubagents(maxSubagentsPath, maxSubagents.ValueInt32(), diags)
	}
}

func durationFromConfig(ctx context.Context, cfg tfsdk.Config, attrPath path.Path, diags *diag.Diagnostics) (*durationpb.Duration, bool) {
	var value types.String
	diags.Append(cfg.GetAttribute(ctx, attrPath, &value)...)
	if diags.HasError() || value.IsNull() || value.IsUnknown() {
		return nil, false
	}
	duration, err := time.ParseDuration(value.ValueString())
	if err != nil {
		diags.AddAttributeError(attrPath, "Invalid Duration", "Use a Go duration string such as \"30m\", \"24h\", or \"0s\".")
		return nil, false
	}
	return durationpb.New(duration), true
}

func boolFromConfig(ctx context.Context, cfg tfsdk.Config, attrPath path.Path, planValue types.Bool, diags *diag.Diagnostics) (*bool, bool) {
	var value types.Bool
	diags.Append(cfg.GetAttribute(ctx, attrPath, &value)...)
	if diags.HasError() || value.IsNull() || value.IsUnknown() {
		return nil, false
	}
	result := planValue.ValueBool()
	return &result, true
}

func int64FromConfig(ctx context.Context, cfg tfsdk.Config, attrPath path.Path, planValue types.Int64, diags *diag.Diagnostics) (*int64, bool) {
	var value types.Int64
	diags.Append(cfg.GetAttribute(ctx, attrPath, &value)...)
	if diags.HasError() || value.IsNull() || value.IsUnknown() {
		return nil, false
	}
	result := planValue.ValueInt64()
	return &result, true
}

func stringFromConfig(ctx context.Context, cfg tfsdk.Config, attrPath path.Path, planValue types.String, diags *diag.Diagnostics) (*string, bool) {
	var value types.String
	diags.Append(cfg.GetAttribute(ctx, attrPath, &value)...)
	if diags.HasError() || value.IsNull() || value.IsUnknown() {
		return nil, false
	}
	result := planValue.ValueString()
	return &result, true
}
