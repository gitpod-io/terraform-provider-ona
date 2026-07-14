// Copyright IBM Corp. 2021, 2026
// SPDX-License-Identifier: MPL-2.0

package project

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"connectrpc.com/connect"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/api/go/client"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &Resource{}
var _ resource.ResourceWithConfigure = &Resource{}
var _ resource.ResourceWithImportState = &Resource{}
var _ resource.ResourceWithValidateConfig = &Resource{}

func NewResource() resource.Resource {
	return &Resource{}
}

type Resource struct {
	client *managementclient.ManagementPlane
}

func (r *Resource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (r *Resource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceSchema()
}

func (r *Resource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *Resource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data ProjectModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateProjectModel(ctx, data, &resp.Diagnostics)
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ProjectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before creating ona_project resources.",
		)
		return
	}

	createReq, diags := projectCreateRequest(ctx, data)
	resp.Diagnostics.Append(diags...)
	environmentClasses, classDiags := projectEnvironmentClassesFromModel(data.EnvironmentClasses, path.Root("environment_class"), false)
	resp.Diagnostics.Append(classDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.ProjectService().CreateProject(ctx, connect.NewRequest(createReq))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Create Ona Project", "creating the Ona project", err)
		return
	}

	data.ID = types.StringValue(result.Msg.GetProject().GetId())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.updateEnvironmentClasses(ctx, data.ID.ValueString(), environmentClasses); err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Update Ona Project Environment Classes", "assigning environment classes to the created Ona project", err)
		return
	}

	project, err := r.getProject(ctx, data.ID.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Created Ona Project", "reading the created Ona project", err)
		return
	}
	if project == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	planned := data
	resp.Diagnostics.Append(populateProjectModel(ctx, &data, project)...)
	preserveProjectPlannedInputs(&data, planned)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ProjectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before reading ona_project resources.",
		)
		return
	}

	id := data.ID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError("Unable to Read Ona Project", "Project ID is empty.")
		return
	}

	project, err := r.getProject(ctx, id)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Project", "reading the Ona project", err)
		return
	}
	if project == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	prior := data
	data = ProjectModel{}
	resp.Diagnostics.Append(populateProjectModel(ctx, &data, project)...)
	preserveProjectPlannedInputs(&data, prior)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ProjectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var prior ProjectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before updating ona_project resources.",
		)
		return
	}

	updateReq, diags := projectUpdateRequest(ctx, data, prior)
	resp.Diagnostics.Append(diags...)
	environmentClasses, classDiags := projectEnvironmentClassesFromModel(data.EnvironmentClasses, path.Root("environment_class"), false)
	resp.Diagnostics.Append(classDiags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.ProjectService().UpdateProject(ctx, connect.NewRequest(updateReq)); err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Update Ona Project", "updating the Ona project", err)
		return
	}
	if err := r.updateEnvironmentClasses(ctx, data.ID.ValueString(), environmentClasses); err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Update Ona Project Environment Classes", "updating Ona project environment classes", err)
		return
	}

	project, err := r.getProject(ctx, data.ID.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Updated Ona Project", "reading the updated Ona project", err)
		return
	}
	if project == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	planned := data
	resp.Diagnostics.Append(populateProjectModel(ctx, &data, project)...)
	preserveProjectPlannedInputs(&data, planned)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ProjectModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before deleting ona_project resources.",
		)
		return
	}

	id := data.ID.ValueString()
	if id == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	_, err := r.client.ProjectService().DeleteProject(ctx, connect.NewRequest(&v1.DeleteProjectRequest{ProjectId: id}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Delete Ona Project", "deleting the Ona project", err)
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *Resource) getProject(ctx context.Context, id string) (*v1.Project, error) {
	result, err := r.client.ProjectService().GetProject(ctx, connect.NewRequest(&v1.GetProjectRequest{ProjectId: id}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get project: %w", err)
	}
	return result.Msg.GetProject(), nil
}

func (r *Resource) updateEnvironmentClasses(ctx context.Context, projectID string, classes []*v1.ProjectEnvironmentClass) error {
	if len(classes) == 0 {
		return invalidMappingError("missing environment classes")
	}
	_, err := r.client.ProjectService().UpdateProjectEnvironmentClasses(ctx, connect.NewRequest(&v1.UpdateProjectEnvironmentClassesRequest{
		ProjectId:                 projectID,
		ProjectEnvironmentClasses: classes,
	}))
	if err != nil {
		return fmt.Errorf("update project environment classes: %w", err)
	}
	return nil
}

func populateProjectModel(ctx context.Context, data *ProjectModel, project *v1.Project) diag.Diagnostics {
	model, diags := projectModelFromProto(ctx, project)
	if diags.HasError() {
		return diags
	}
	*data = model
	return diags
}

func validateProjectModel(ctx context.Context, data ProjectModel, diags *diag.Diagnostics) {
	validateName(data.Name, diags)
	validateRequiredString(data.RepositoryCloneURL, path.Root("repository_clone_url"), "Repository Clone URL", diags)
	validateRepositoryCloneURL(data.RepositoryCloneURL, diags)
	validateRequiredString(data.Branch, path.Root("branch"), "Branch", diags)
	validateRelativePath(data.DevcontainerFilePath, path.Root("devcontainer_file_path"), "devcontainer_file_path", diags)
	validateRelativePath(data.AutomationsFilePath, path.Root("automations_file_path"), "automations_file_path", diags)
	_, classDiags := projectEnvironmentClassesFromModel(data.EnvironmentClasses, path.Root("environment_class"), true)
	diags.Append(classDiags...)
	if len(data.Prebuild) > 0 {
		_, prebuildDiags := prebuildConfigurationFromModel(ctx, data.Prebuild, path.Root("prebuild_configuration"), true)
		diags.Append(prebuildDiags...)
	}
}

func validateName(value types.String, diags *diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return
	}
	name := value.ValueString()
	if len(name) < 1 || len(name) > 80 {
		diags.AddAttributeError(path.Root("name"), "Invalid Project Name", "Project name must be between 1 and 80 characters.")
	}
}

func validateRequiredString(value types.String, p path.Path, name string, diags *diag.Diagnostics) {
	if value.IsUnknown() {
		return
	}
	if value.IsNull() || strings.TrimSpace(value.ValueString()) == "" {
		diags.AddAttributeError(p, "Missing Project "+name, name+" must not be empty.")
	}
}

var scpLikeGitURLPattern = regexp.MustCompile(`^[^@\s]+@[^:\s]+:.+$`)

func validateRepositoryCloneURL(value types.String, diags *diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() || value.ValueString() == "" {
		return
	}
	raw := strings.TrimSpace(value.ValueString())
	if raw != value.ValueString() {
		diags.AddAttributeError(path.Root("repository_clone_url"), "Invalid Project Repository Clone URL", "repository_clone_url must not include leading or trailing whitespace.")
		return
	}
	if scpLikeGitURLPattern.MatchString(raw) {
		return
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.Path == "" || parsed.Path == "/" {
		diags.AddAttributeError(path.Root("repository_clone_url"), "Invalid Project Repository Clone URL", "Use an HTTP(S) or SSH Git clone URL, such as https://github.com/ona/example.git or git@github.com:ona/example.git.")
		return
	}
	switch parsed.Scheme {
	case "http", "https", "ssh", "git":
	default:
		diags.AddAttributeError(path.Root("repository_clone_url"), "Invalid Project Repository Clone URL", "Supported clone URL schemes are http, https, ssh, and git.")
	}
}

func validateRelativePath(value types.String, p path.Path, name string, diags *diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return
	}
	raw := value.ValueString()
	if raw == "" {
		return
	}
	if raw[0] == '/' || strings.Contains(raw, `\`) || strings.Contains(raw, "://") {
		diags.AddAttributeError(p, "Invalid Project File Path", fmt.Sprintf("%s must be a repository-relative path using forward slashes.", name))
		return
	}
	for _, segment := range strings.Split(raw, "/") {
		if segment == ".." {
			diags.AddAttributeError(p, "Invalid Project File Path", fmt.Sprintf("%s must stay within the repository and must not contain .. path segments.", name))
			return
		}
	}
}
