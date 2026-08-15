// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package accesscontrol

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/tfvalue"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const runnerRoleAssignmentPageSize int32 = 100

var _ resource.Resource = &RunnerRoleAssignmentResource{}
var _ resource.ResourceWithConfigure = &RunnerRoleAssignmentResource{}
var _ resource.ResourceWithIdentity = &RunnerRoleAssignmentResource{}
var _ resource.ResourceWithImportState = &RunnerRoleAssignmentResource{}
var _ resource.ResourceWithValidateConfig = &RunnerRoleAssignmentResource{}

func NewRunnerRoleAssignmentResource() resource.Resource {
	return &RunnerRoleAssignmentResource{}
}

type RunnerRoleAssignmentResource struct {
	client *managementclient.ManagementPlane
}

type RunnerRoleAssignmentModel struct {
	ID       types.String `tfsdk:"id"`
	RunnerID types.String `tfsdk:"runner_id"`
	GroupID  types.String `tfsdk:"group_id"`
	Role     types.String `tfsdk:"role"`
}

type runnerRoleAssignmentMatches struct {
	direct  []*v1.RoleAssignment
	derived []*v1.RoleAssignment
}

var runnerRoleToAPI = map[string]v1.ResourceRole{
	"admin": v1.ResourceRole_RESOURCE_ROLE_RUNNER_ADMIN,
	"user":  v1.ResourceRole_RESOURCE_ROLE_RUNNER_USER,
}

var apiToRunnerRole = func() map[v1.ResourceRole]string {
	result := make(map[v1.ResourceRole]string, len(runnerRoleToAPI))
	for role, apiRole := range runnerRoleToAPI {
		result[apiRole] = role
	}
	return result
}()

func (r *RunnerRoleAssignmentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runner_role_assignment"
}

func (r *RunnerRoleAssignmentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		MarkdownDescription: "Ona role assignment granting a group access to one runner.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Role assignment ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"runner_id": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Runner ID receiving the group access. Changing this value replaces the assignment.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"group_id": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Group ID receiving access to the runner. Changing this value replaces the assignment.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"role": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Runner role. Supported values are `user` and `admin`. Changing this value replaces the assignment.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *RunnerRoleAssignmentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = providerdata.ResourceClient(req.ProviderData, r.client, &resp.Diagnostics)
}

func (r *RunnerRoleAssignmentResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data RunnerRoleAssignmentModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateConfiguredRunnerRoleAssignmentID(path.Root("runner_id"), "runner_id", data.RunnerID, false, &resp.Diagnostics)
	validateConfiguredRunnerRoleAssignmentID(path.Root("group_id"), "group_id", data.GroupID, false, &resp.Diagnostics)
	validateRunnerRole(data.Role, &resp.Diagnostics)
}

func (r *RunnerRoleAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data RunnerRoleAssignmentModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateConfiguredRunnerRoleAssignmentID(path.Root("runner_id"), "runner_id", data.RunnerID, true, &resp.Diagnostics)
	validateConfiguredRunnerRoleAssignmentID(path.Root("group_id"), "group_id", data.GroupID, true, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "creating", "ona_runner_role_assignment") {
		return
	}

	apiRole, ok := runnerRoleToAPI[data.Role.ValueString()]
	if !ok {
		addInvalidRunnerRoleDiagnostic(path.Root("role"), data.Role.ValueString(), &resp.Diagnostics)
		return
	}

	matches, err := r.listAssignments(ctx, data.RunnerID.ValueString(), data.GroupID.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Create Ona Runner Role Assignment", "checking for an existing runner role assignment", err)
		return
	}
	if len(matches.direct) > 1 {
		resp.Diagnostics.AddError(
			"Unable to Create Ona Runner Role Assignment",
			fmt.Sprintf("Multiple direct runner role assignments already exist for runner %q and group %q: %s. Remove the duplicates before managing this access with Terraform.", data.RunnerID.ValueString(), data.GroupID.ValueString(), formatRunnerRoleAssignmentIDs(matches.direct)),
		)
		return
	}
	if len(matches.direct) == 1 {
		existing := matches.direct[0]
		role := existing.GetResourceRole().String()
		if terraformRole, supported := apiToRunnerRole[existing.GetResourceRole()]; supported {
			role = terraformRole
		}
		resp.Diagnostics.AddError(
			"Unable to Create Ona Runner Role Assignment",
			fmt.Sprintf("A direct runner role assignment for runner %q and group %q already exists with ID %q and role %q. Import it with %q instead of creating a duplicate.", data.RunnerID.ValueString(), data.GroupID.ValueString(), existing.GetId(), role, data.RunnerID.ValueString()+"/"+data.GroupID.ValueString()),
		)
		return
	}
	if len(matches.derived) > 0 {
		resp.Diagnostics.AddError(
			"Unable to Create Ona Runner Role Assignment",
			fmt.Sprintf("Access to runner %q for group %q is derived from an organization role: %s. Manage the organization role instead of importing or replacing the derived assignment.", data.RunnerID.ValueString(), data.GroupID.ValueString(), formatDerivedRunnerRoleAssignments(matches.derived)),
		)
		return
	}

	result, err := r.client.GroupService().CreateRoleAssignment(ctx, connect.NewRequest(&v1.CreateRoleAssignmentRequest{
		GroupId:      data.GroupID.ValueString(),
		ResourceType: v1.ResourceType_RESOURCE_TYPE_RUNNER,
		ResourceId:   data.RunnerID.ValueString(),
		ResourceRole: apiRole,
	}))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Create Ona Runner Role Assignment", "creating the runner role assignment", err)
		return
	}
	if result == nil || result.Msg == nil || result.Msg.GetAssignment() == nil {
		r.recoverMalformedCreate(ctx, &data, apiRole, "The Ona API returned an empty role assignment.", resp)
		return
	}

	assignment := result.Msg.GetAssignment()
	if assignment.GetId() == "" {
		r.recoverMalformedCreate(ctx, &data, apiRole, "The Ona API returned a role assignment without an ID.", resp)
		return
	}

	data.ID = types.StringValue(assignment.GetId())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, RunnerRoleAssignmentIdentityModel{
		RunnerID: data.RunnerID,
		GroupID:  data.GroupID,
	})...)
	if resp.Diagnostics.HasError() {
		return
	}

	observed, err := runnerRoleAssignmentModelFromAPI(assignment)
	if err != nil {
		resp.Diagnostics.AddError("Unable to Create Ona Runner Role Assignment", fmt.Sprintf("The Ona API returned a malformed role assignment: %v.", err))
		return
	}
	if !matchesRunnerRoleAssignmentScope(assignment, data.RunnerID.ValueString(), data.GroupID.ValueString()) || assignment.GetResourceRole() != apiRole {
		resp.Diagnostics.AddError("Unable to Create Ona Runner Role Assignment", "The Ona API returned a role assignment that does not match the requested runner, group, and role.")
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &observed)...)
}

func (r *RunnerRoleAssignmentResource) recoverMalformedCreate(ctx context.Context, data *RunnerRoleAssignmentModel, apiRole v1.ResourceRole, malformedResponseDetail string, resp *resource.CreateResponse) {
	matches, err := r.listAssignments(ctx, data.RunnerID.ValueString(), data.GroupID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Ona Runner Role Assignment",
			fmt.Sprintf("%s The provider could not recover the created assignment: %v.", malformedResponseDetail, err),
		)
		return
	}
	assignment, err := selectCreatedRunnerRoleAssignment(matches.direct, apiRole)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Ona Runner Role Assignment",
			fmt.Sprintf("%s The provider could not recover the created assignment: %v.", malformedResponseDetail, err),
		)
		return
	}

	data.ID = types.StringValue(assignment.GetId())
	resp.Diagnostics.Append(resp.State.Set(ctx, data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, RunnerRoleAssignmentIdentityModel{
		RunnerID: data.RunnerID,
		GroupID:  data.GroupID,
	})...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.AddError(
		"Unable to Create Ona Runner Role Assignment",
		fmt.Sprintf("%s Terraform recovered assignment ID %q in state so the assignment can be safely retried or destroyed.", malformedResponseDetail, assignment.GetId()),
	)
}

func (r *RunnerRoleAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data RunnerRoleAssignmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "reading", "ona_runner_role_assignment") {
		return
	}

	matches, err := r.listAssignments(ctx, data.RunnerID.ValueString(), data.GroupID.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Runner Role Assignment", "reading the runner role assignment", err)
		return
	}
	if len(matches.direct) > 1 {
		resp.Diagnostics.AddError(
			"Unable to Read Ona Runner Role Assignment",
			fmt.Sprintf("Multiple direct runner role assignments exist for runner %q and group %q: %s. Remove the duplicates before refreshing this resource.", data.RunnerID.ValueString(), data.GroupID.ValueString(), formatRunnerRoleAssignmentIDs(matches.direct)),
		)
		return
	}
	if len(matches.direct) == 0 {
		if len(matches.derived) > 0 {
			resp.Diagnostics.AddError(
				"Unable to Read Ona Runner Role Assignment",
				fmt.Sprintf("Access to runner %q for group %q is derived from an organization role: %s. Terraform cannot manage the derived assignment.", data.RunnerID.ValueString(), data.GroupID.ValueString(), formatDerivedRunnerRoleAssignments(matches.derived)),
			)
			return
		}
		resp.State.RemoveResource(ctx)
		return
	}

	observed, err := runnerRoleAssignmentModelFromAPI(matches.direct[0])
	if err != nil {
		resp.Diagnostics.AddError("Unable to Read Ona Runner Role Assignment", fmt.Sprintf("The Ona API returned a malformed role assignment: %v.", err))
		return
	}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, RunnerRoleAssignmentIdentityModel{
		RunnerID: observed.RunnerID,
		GroupID:  observed.GroupID,
	})...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &observed)...)
}

func (r *RunnerRoleAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Unable to Update Ona Runner Role Assignment", "Runner role assignments are immutable. Change runner_id, group_id, or role by replacing the resource.")
}

func (r *RunnerRoleAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data RunnerRoleAssignmentModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireResourceClient(r.client, &resp.Diagnostics, "deleting", "ona_runner_role_assignment") {
		return
	}
	if data.ID.IsNull() || data.ID.IsUnknown() || data.ID.ValueString() == "" {
		resp.Diagnostics.AddError("Unable to Delete Ona Runner Role Assignment", "Terraform state does not contain the role assignment ID required for deletion. Refresh or re-import the resource before retrying.")
		return
	}

	_, err := r.client.GroupService().DeleteRoleAssignment(ctx, connect.NewRequest(&v1.DeleteRoleAssignmentRequest{
		AssignmentId: data.ID.ValueString(),
	}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Delete Ona Runner Role Assignment", "deleting the runner role assignment", err)
		return
	}
	resp.State.RemoveResource(ctx)
}

func (r *RunnerRoleAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID == "" {
		var identity RunnerRoleAssignmentIdentityModel
		resp.Diagnostics.Append(req.Identity.Get(ctx, &identity)...)
		if resp.Diagnostics.HasError() {
			return
		}
		validateRunnerRoleAssignmentIdentity(identity, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
		tfvalue.SetImportString(ctx, resp, "runner_id", identity.RunnerID.ValueString())
		if resp.Diagnostics.HasError() {
			return
		}
		tfvalue.SetImportString(ctx, resp, "group_id", identity.GroupID.ValueString())
		if resp.Diagnostics.HasError() {
			return
		}
		resp.Diagnostics.Append(resp.Identity.Set(ctx, identity)...)
		return
	}

	parts, diags := splitRunnerRoleAssignmentImportID(req.ID)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	tfvalue.SetImportString(ctx, resp, "runner_id", parts[0])
	if resp.Diagnostics.HasError() {
		return
	}
	tfvalue.SetImportString(ctx, resp, "group_id", parts[1])
}

func (r *RunnerRoleAssignmentResource) listAssignments(ctx context.Context, runnerID, groupID string) (runnerRoleAssignmentMatches, error) {
	var matches runnerRoleAssignmentMatches
	var token string
	seenTokens := make(map[string]struct{})
	for {
		result, err := r.client.GroupService().ListRoleAssignments(ctx, connect.NewRequest(&v1.ListRoleAssignmentsRequest{
			Pagination: &v1.PaginationRequest{PageSize: runnerRoleAssignmentPageSize, Token: token},
			Filter: &v1.ListRoleAssignmentsRequest_Filter{
				GroupId:       groupID,
				ResourceTypes: []v1.ResourceType{v1.ResourceType_RESOURCE_TYPE_RUNNER},
				ResourceId:    runnerID,
			},
		}))
		if err != nil {
			return runnerRoleAssignmentMatches{}, fmt.Errorf("list role assignments: %w", err)
		}
		if result == nil || result.Msg == nil {
			return runnerRoleAssignmentMatches{}, fmt.Errorf("list role assignments: Ona API returned an empty response")
		}

		pageMatches := classifyRunnerRoleAssignments(result.Msg.GetAssignments(), runnerID, groupID)
		matches.direct = append(matches.direct, pageMatches.direct...)
		matches.derived = append(matches.derived, pageMatches.derived...)

		nextToken := result.Msg.GetPagination().GetNextToken()
		if nextToken == "" {
			return matches, nil
		}
		if _, ok := seenTokens[nextToken]; ok {
			return runnerRoleAssignmentMatches{}, fmt.Errorf("list role assignments: Ona API returned repeated pagination token %q", nextToken)
		}
		seenTokens[nextToken] = struct{}{}
		token = nextToken
	}
}

func runnerRoleAssignmentModelFromAPI(assignment *v1.RoleAssignment) (RunnerRoleAssignmentModel, error) {
	if assignment == nil {
		return RunnerRoleAssignmentModel{}, fmt.Errorf("role assignment is missing")
	}
	if assignment.GetId() == "" {
		return RunnerRoleAssignmentModel{}, fmt.Errorf("role assignment ID is empty")
	}
	if assignment.GetGroupId() == "" {
		return RunnerRoleAssignmentModel{}, fmt.Errorf("role assignment group ID is empty")
	}
	if assignment.GetResourceId() == "" {
		return RunnerRoleAssignmentModel{}, fmt.Errorf("role assignment runner ID is empty")
	}
	if assignment.GetResourceType() != v1.ResourceType_RESOURCE_TYPE_RUNNER {
		return RunnerRoleAssignmentModel{}, fmt.Errorf("role assignment resource type is %s, expected RESOURCE_TYPE_RUNNER", assignment.GetResourceType())
	}
	if isDerivedRunnerRoleAssignment(assignment) {
		return RunnerRoleAssignmentModel{}, fmt.Errorf("role assignment is derived from organization role %s", assignment.GetDerivedFromOrgRole())
	}
	role, ok := apiToRunnerRole[assignment.GetResourceRole()]
	if !ok {
		return RunnerRoleAssignmentModel{}, fmt.Errorf("role assignment has unsupported runner role %s", assignment.GetResourceRole())
	}

	return RunnerRoleAssignmentModel{
		ID:       types.StringValue(assignment.GetId()),
		RunnerID: types.StringValue(assignment.GetResourceId()),
		GroupID:  types.StringValue(assignment.GetGroupId()),
		Role:     types.StringValue(role),
	}, nil
}

func classifyRunnerRoleAssignments(assignments []*v1.RoleAssignment, runnerID, groupID string) runnerRoleAssignmentMatches {
	var matches runnerRoleAssignmentMatches
	for _, assignment := range assignments {
		if !matchesRunnerRoleAssignmentScope(assignment, runnerID, groupID) {
			continue
		}
		if isDerivedRunnerRoleAssignment(assignment) {
			matches.derived = append(matches.derived, assignment)
			continue
		}
		matches.direct = append(matches.direct, assignment)
	}
	return matches
}

func matchesRunnerRoleAssignmentScope(assignment *v1.RoleAssignment, runnerID, groupID string) bool {
	return assignment != nil &&
		assignment.GetGroupId() == groupID &&
		assignment.GetResourceType() == v1.ResourceType_RESOURCE_TYPE_RUNNER &&
		assignment.GetResourceId() == runnerID
}

func isDerivedRunnerRoleAssignment(assignment *v1.RoleAssignment) bool {
	return assignment != nil &&
		assignment.DerivedFromOrgRole != nil &&
		assignment.GetDerivedFromOrgRole() != v1.ResourceRole_RESOURCE_ROLE_UNSPECIFIED
}

func selectCreatedRunnerRoleAssignment(assignments []*v1.RoleAssignment, role v1.ResourceRole) (*v1.RoleAssignment, error) {
	var matches []*v1.RoleAssignment
	for _, assignment := range assignments {
		if assignment.GetResourceRole() == role {
			matches = append(matches, assignment)
		}
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no direct assignment with role %s was found", role)
	}
	if len(matches) > 1 {
		return nil, fmt.Errorf("multiple direct assignments with role %s were found: %s", role, formatRunnerRoleAssignmentIDs(matches))
	}
	if matches[0].GetId() == "" {
		return nil, fmt.Errorf("the matching direct assignment has an empty ID")
	}
	return matches[0], nil
}

func formatRunnerRoleAssignmentIDs(assignments []*v1.RoleAssignment) string {
	ids := make([]string, 0, len(assignments))
	for _, assignment := range assignments {
		id := assignment.GetId()
		if id == "" {
			id = "<empty>"
		}
		ids = append(ids, fmt.Sprintf("%q", id))
	}
	sort.Strings(ids)
	return strings.Join(ids, ", ")
}

func formatDerivedRunnerRoleAssignments(assignments []*v1.RoleAssignment) string {
	descriptions := make([]string, 0, len(assignments))
	for _, assignment := range assignments {
		descriptions = append(descriptions, fmt.Sprintf("ID %q from %s", assignment.GetId(), assignment.GetDerivedFromOrgRole()))
	}
	sort.Strings(descriptions)
	return strings.Join(descriptions, ", ")
}

func splitRunnerRoleAssignmentImportID(id string) ([]string, diag.Diagnostics) {
	parts, diags := tfvalue.SplitImportID(id, 2, "runner_id/group_id")
	if diags.HasError() {
		return nil, diags
	}
	for index, name := range []string{"runner_id", "group_id"} {
		if _, err := uuid.Parse(parts[index]); err != nil {
			diags.AddError("Invalid Import ID", fmt.Sprintf("%s must be a valid UUID.", name))
		}
	}
	if diags.HasError() {
		return nil, diags
	}
	return parts, diags
}

func validateRunnerRoleAssignmentIdentity(identity RunnerRoleAssignmentIdentityModel, diags *diag.Diagnostics) {
	validateRunnerRoleAssignmentIdentityString(path.Root("runner_id"), "runner_id", identity.RunnerID, diags)
	validateRunnerRoleAssignmentIdentityString(path.Root("group_id"), "group_id", identity.GroupID, diags)
}

func validateRunnerRoleAssignmentIdentityString(attrPath path.Path, name string, value types.String, diags *diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() || strings.TrimSpace(value.ValueString()) == "" {
		diags.AddAttributeError(attrPath, "Invalid Runner Role Assignment Identity", fmt.Sprintf("%s must be a non-empty string.", name))
		return
	}
	if _, err := uuid.Parse(value.ValueString()); err != nil {
		diags.AddAttributeError(attrPath, "Invalid Runner Role Assignment Identity", fmt.Sprintf("%s must be a valid UUID.", name))
	}
}

func validateConfiguredRunnerRoleAssignmentID(attrPath path.Path, name string, value types.String, requireKnown bool, diags *diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		if requireKnown {
			diags.AddAttributeError(attrPath, "Invalid Runner Role Assignment Configuration", fmt.Sprintf("%s must be a known UUID before creating the assignment.", name))
		}
		return
	}
	if _, err := uuid.Parse(value.ValueString()); err != nil {
		diags.AddAttributeError(attrPath, "Invalid Runner Role Assignment Configuration", fmt.Sprintf("%s must be a valid UUID.", name))
	}
}

func validateRunnerRole(role types.String, diags *diag.Diagnostics) {
	if role.IsNull() || role.IsUnknown() {
		return
	}
	if _, ok := runnerRoleToAPI[role.ValueString()]; !ok {
		addInvalidRunnerRoleDiagnostic(path.Root("role"), role.ValueString(), diags)
	}
}

func addInvalidRunnerRoleDiagnostic(attrPath path.Path, role string, diags *diag.Diagnostics) {
	diags.AddAttributeError(
		attrPath,
		"Unsupported Runner Role",
		fmt.Sprintf("Unsupported runner role %q. Supported values are: %s.", role, strings.Join(supportedRunnerRoles(), ", ")),
	)
}

func supportedRunnerRoles() []string {
	roles := make([]string, 0, len(runnerRoleToAPI))
	for role := range runnerRoleToAPI {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	return roles
}
