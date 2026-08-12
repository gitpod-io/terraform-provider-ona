// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package project

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &ProjectDataSource{}
var _ datasource.DataSourceWithConfigure = &ProjectDataSource{}

func NewProjectDataSource() datasource.DataSource {
	return &ProjectDataSource{}
}

type ProjectDataSource struct {
	client *managementclient.ManagementPlane
}

type ProjectDataSourceModel struct {
	ID                   types.String                 `tfsdk:"id"`
	ProjectID            types.String                 `tfsdk:"project_id"`
	Name                 types.String                 `tfsdk:"name"`
	RepositoryCloneURL   types.String                 `tfsdk:"repository_clone_url"`
	Branch               types.String                 `tfsdk:"branch"`
	InsightsEnabled      types.Bool                   `tfsdk:"insights_enabled"`
	DevcontainerFilePath types.String                 `tfsdk:"devcontainer_file_path"`
	AutomationsFilePath  types.String                 `tfsdk:"automations_file_path"`
	EnvironmentClasses   []EnvironmentClassModel      `tfsdk:"environment_class"`
	Prebuild             []PrebuildConfigurationModel `tfsdk:"prebuild_configuration"`
	CreatedAt            types.String                 `tfsdk:"created_at"`
	Creator              types.Object                 `tfsdk:"creator"`
}

func (d *ProjectDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (d *ProjectDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dataSourceSchema()
}

func (d *ProjectDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	data, ok := req.ProviderData.(*providerdata.Data)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *providerdata.Data, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = data.Client
}

func (d *ProjectDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ProjectDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if d.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before reading ona_project data sources.",
		)
		return
	}

	if data.ProjectID.IsNull() || data.ProjectID.IsUnknown() || strings.TrimSpace(data.ProjectID.ValueString()) == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("project_id"),
			"Missing Ona Project ID",
			"project_id must be known and non-empty before reading the data source.",
		)
		return
	}
	projectID := data.ProjectID.ValueString()

	result, err := d.client.ProjectService().GetProject(ctx, connect.NewRequest(&v1.GetProjectRequest{ProjectId: projectID}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			resp.Diagnostics.AddAttributeError(
				path.Root("project_id"),
				"Ona Project Not Found or Not Visible",
				fmt.Sprintf("No Ona project visible to the configured token was found for project_id %q.", projectID),
			)
			return
		}
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Project", "reading the Ona project data source", err)
		return
	}
	if result == nil || result.Msg == nil || result.Msg.GetProject() == nil {
		resp.Diagnostics.AddError("Unable to Read Ona Project", "The Ona API returned an empty project response.")
		return
	}

	projectModel, diags := projectModelFromProto(ctx, result.Msg.GetProject())
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectModel.InsightsEnabled, err = projectInsightsEnabled(ctx, d.client, projectID)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Project Insights", "reading project Insights enablement for the Ona project data source", err)
		return
	}

	data = projectDataSourceModel(projectModel)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func projectDataSourceModel(model ProjectModel) ProjectDataSourceModel {
	return ProjectDataSourceModel{
		ID:                   model.ID,
		ProjectID:            model.ID,
		Name:                 model.Name,
		RepositoryCloneURL:   model.RepositoryCloneURL,
		Branch:               model.Branch,
		InsightsEnabled:      model.InsightsEnabled,
		DevcontainerFilePath: model.DevcontainerFilePath,
		AutomationsFilePath:  model.AutomationsFilePath,
		EnvironmentClasses:   model.EnvironmentClasses,
		Prebuild:             model.Prebuild,
		CreatedAt:            model.CreatedAt,
		Creator:              model.Creator,
	}
}

func projectInsightsEnabled(ctx context.Context, client *managementclient.ManagementPlane, projectID string) (types.Bool, error) {
	result, err := client.InsightsService().GetProjectInsightsStatus(ctx, connect.NewRequest(&v1.GetProjectInsightsStatusRequest{ProjectId: projectID}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return types.BoolValue(false), nil
		}
		return types.BoolNull(), fmt.Errorf("get project insights status: %w", err)
	}
	if result == nil || result.Msg == nil {
		return types.BoolNull(), fmt.Errorf("get project insights status: Ona API returned an empty response")
	}
	return types.BoolValue(result.Msg.GetEnabled()), nil
}
