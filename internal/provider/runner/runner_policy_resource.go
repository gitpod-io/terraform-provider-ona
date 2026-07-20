// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package runner

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &PolicyResource{}
var _ resource.ResourceWithConfigure = &PolicyResource{}
var _ resource.ResourceWithImportState = &PolicyResource{}
var _ resource.ResourceWithValidateConfig = &PolicyResource{}

const runnerPolicyRoleUser = "user"

var runnerPolicyRoleToAPI = map[string]v1.RunnerRole{
	runnerPolicyRoleUser: v1.RunnerRole_RUNNER_ROLE_USER,
}

var apiToRunnerPolicyRole = func() map[v1.RunnerRole]string {
	result := make(map[v1.RunnerRole]string, len(runnerPolicyRoleToAPI))
	for role, apiRole := range runnerPolicyRoleToAPI {
		result[apiRole] = role
	}
	return result
}()

func NewPolicyResource() resource.Resource {
	return &PolicyResource{}
}

type PolicyResource struct {
	client *managementclient.ManagementPlane
}

type PolicyModel struct {
	ID       types.String `tfsdk:"id"`
	RunnerID types.String `tfsdk:"runner_id"`
	GroupID  types.String `tfsdk:"group_id"`
	Role     types.String `tfsdk:"role"`
}

func (r *PolicyResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runner_policy"
}

func (r *PolicyResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		MarkdownDescription: "Ona runner policy granting a group access to a runner.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Terraform resource ID in `runner_id/group_id` format.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"runner_id": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Runner ID this policy grants access to.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"group_id": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Group ID receiving access to the runner.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"role": resourceschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString(runnerPolicyRoleUser),
				MarkdownDescription: "Runner role granted to the group. The only supported value is `user`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *PolicyResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	data, ok := req.ProviderData.(*providerdata.Data)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *providerdata.Data, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = data.Client
}

func (r *PolicyResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data PolicyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateRunnerPolicyRole(data.Role, &resp.Diagnostics)
}

func (r *PolicyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data PolicyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "creating", "ona_runner_policy") {
		return
	}

	apiRole, ok := runnerPolicyRoleToAPI[data.Role.ValueString()]
	if !ok {
		addInvalidRunnerPolicyRoleDiagnostic(path.Root("role"), data.Role.ValueString(), &resp.Diagnostics)
		return
	}

	result, err := r.client.RunnerService().CreateRunnerPolicy(ctx, connect.NewRequest(&v1.CreateRunnerPolicyRequest{
		RunnerId: data.RunnerID.ValueString(),
		GroupId:  data.GroupID.ValueString(),
		Role:     apiRole,
	}))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Create Ona Runner Policy", "creating the Ona runner policy", err)
		return
	}

	data.ID = types.StringValue(runnerPolicyID(data.RunnerID.ValueString(), data.GroupID.ValueString()))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	policy := result.Msg.GetPolicy()
	if policy == nil {
		resp.Diagnostics.AddError("Unable to Create Ona Runner Policy", "The Ona API returned an empty runner policy.")
		return
	}

	diags := populateRunnerPolicyModel(&data, data.RunnerID.ValueString(), policy)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PolicyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data PolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "reading", "ona_runner_policy") {
		return
	}

	runnerID, groupID, diags := runnerPolicyKeys(data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	policy, err := r.findRunnerPolicy(ctx, runnerID, groupID)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Runner Policy", "reading the Ona runner policy", err)
		return
	}
	if policy == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data = PolicyModel{}
	resp.Diagnostics.Append(populateRunnerPolicyModel(&data, runnerID, policy)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *PolicyResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unable to Update Ona Runner Policy", "Runner policies are immutable. Change runner_id, group_id, or role by replacing the resource.")
}

func (r *PolicyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data PolicyModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "deleting", "ona_runner_policy") {
		return
	}

	runnerID, groupID, diags := runnerPolicyKeys(data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err := r.client.RunnerService().DeleteRunnerPolicy(ctx, connect.NewRequest(&v1.DeleteRunnerPolicyRequest{
		RunnerId: runnerID,
		GroupId:  groupID,
	}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Delete Ona Runner Policy", "deleting the Ona runner policy", err)
		return
	}
	resp.State.RemoveResource(ctx)
}

func (r *PolicyResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts, diags := parseRunnerPolicyImportID(req.ID)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(req.ID))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("runner_id"), types.StringValue(parts[0]))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("group_id"), types.StringValue(parts[1]))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("role"), types.StringValue(runnerPolicyRoleUser))...)
}

func (r *PolicyResource) requireClient(resp *diag.Diagnostics, action string, resourceType string) bool {
	if r.client != nil {
		return true
	}
	resp.AddError(
		"Ona API Client Is Not Configured",
		fmt.Sprintf("Set the provider token argument or ONA_TOKEN before %s %s resources.", action, resourceType),
	)
	return false
}

func (r *PolicyResource) findRunnerPolicy(ctx context.Context, runnerID string, groupID string) (*v1.RunnerPolicy, error) {
	var token string
	for {
		result, err := r.client.RunnerService().ListRunnerPolicies(ctx, connect.NewRequest(&v1.ListRunnerPoliciesRequest{
			Pagination: &v1.PaginationRequest{
				PageSize: 100,
				Token:    token,
			},
			RunnerId: runnerID,
		}))
		if err != nil {
			if connect.CodeOf(err) == connect.CodeNotFound {
				return nil, nil
			}
			return nil, fmt.Errorf("list runner policies: %w", err)
		}
		for _, policy := range result.Msg.GetPolicies() {
			if policy.GetGroupId() == groupID {
				return policy, nil
			}
		}
		token = result.Msg.GetPagination().GetNextToken()
		if token == "" {
			return nil, nil
		}
	}
}

func populateRunnerPolicyModel(data *PolicyModel, runnerID string, policy *v1.RunnerPolicy) diag.Diagnostics {
	var diags diag.Diagnostics
	role, ok := apiToRunnerPolicyRole[policy.GetRole()]
	if !ok {
		addUnsupportedAPIRunnerPolicyRoleDiagnostic(policy.GetRole(), &diags)
		return diags
	}

	data.ID = types.StringValue(runnerPolicyID(runnerID, policy.GetGroupId()))
	data.RunnerID = types.StringValue(runnerID)
	data.GroupID = types.StringValue(policy.GetGroupId())
	data.Role = types.StringValue(role)
	return diags
}

func validateRunnerPolicyRole(role types.String, diags *diag.Diagnostics) {
	if role.IsNull() || role.IsUnknown() {
		return
	}
	if _, ok := runnerPolicyRoleToAPI[role.ValueString()]; !ok {
		addInvalidRunnerPolicyRoleDiagnostic(path.Root("role"), role.ValueString(), diags)
	}
}

func parseRunnerPolicyImportID(id string) ([]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	parts := strings.Split(id, "/")
	if len(parts) != 2 {
		diags.AddError("Invalid Import ID", "Expected import ID format: runner_id/group_id.")
		return nil, diags
	}
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			diags.AddError("Invalid Import ID", "Expected import ID format: runner_id/group_id.")
			return nil, diags
		}
	}
	return parts, diags
}

func runnerPolicyKeys(data PolicyModel) (string, string, diag.Diagnostics) {
	var diags diag.Diagnostics
	runnerID := data.RunnerID.ValueString()
	groupID := data.GroupID.ValueString()
	if runnerID == "" || groupID == "" {
		parts, parseDiags := parseRunnerPolicyImportID(data.ID.ValueString())
		diags.Append(parseDiags...)
		if diags.HasError() {
			return "", "", diags
		}
		runnerID = parts[0]
		groupID = parts[1]
	}
	return runnerID, groupID, diags
}

func runnerPolicyID(runnerID string, groupID string) string {
	return runnerID + "/" + groupID
}

func addInvalidRunnerPolicyRoleDiagnostic(attrPath path.Path, role string, diags *diag.Diagnostics) {
	diags.AddAttributeError(
		attrPath,
		"Unsupported Runner Policy Role",
		fmt.Sprintf("Unsupported runner policy role %q. Supported values are: %s.", role, strings.Join(supportedRunnerPolicyRoles(), ", ")),
	)
}

func addUnsupportedAPIRunnerPolicyRoleDiagnostic(role v1.RunnerRole, diags *diag.Diagnostics) {
	diags.AddError(
		"Unsupported Ona Runner Policy Role",
		fmt.Sprintf("The Ona API returned unsupported runner policy role %q. Supported values are: %s.", role.String(), strings.Join(supportedRunnerPolicyRoles(), ", ")),
	)
}

func supportedRunnerPolicyRoles() []string {
	roles := make([]string, 0, len(runnerPolicyRoleToAPI))
	for role := range runnerPolicyRoleToAPI {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	return roles
}
