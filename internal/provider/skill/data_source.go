// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package skill

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &DataSource{}
var _ datasource.DataSourceWithConfigure = &DataSource{}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

type DataSource struct {
	client *managementclient.ManagementPlane
}

type DataSourceModel struct {
	SkillID     types.String `tfsdk:"skill_id"`
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Prompt      types.String `tfsdk:"prompt"`
	Command     types.String `tfsdk:"command"`
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_skill"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = dataSourceSchema()
}

func (d *DataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = providerdata.DataSourceClient(req.ProviderData, d.client, &resp.Diagnostics)
}

func (d *DataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !providerdata.RequireDataSourceClient(d.client, &resp.Diagnostics, "ona_skill") {
		return
	}
	if data.SkillID.IsNull() || data.SkillID.IsUnknown() {
		resp.Diagnostics.AddAttributeError(path.Root("skill_id"), "Missing Skill ID", "skill_id must be known before reading the data source.")
		return
	}
	skillID := data.SkillID.ValueString()
	if _, err := uuid.Parse(skillID); err != nil {
		resp.Diagnostics.AddAttributeError(path.Root("skill_id"), "Invalid Skill ID", "skill_id must be a valid Prompt UUID.")
		return
	}

	organizationID, err := providerdata.AuthenticatedOrganizationID(ctx, d.client)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Ona Organization", "getting the authenticated organization for the ona_skill data source", err)
		return
	}
	result, err := d.client.AgentService().GetPrompt(ctx, connect.NewRequest(&v1.GetPromptRequest{PromptId: skillID}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			resp.Diagnostics.AddAttributeError(path.Root("skill_id"), "Ona Skill Not Found", fmt.Sprintf("No Ona skill visible to the configured token was found for skill_id %q.", skillID))
			return
		}
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Skill", "reading the Ona skill data source", err)
		return
	}
	if result == nil || result.Msg == nil {
		resp.Diagnostics.AddError("Unable to Read Ona Skill", "The Ona API returned an empty response.")
		return
	}
	mapped, mappingDiags := modelFromPrompt(result.Msg.GetPrompt(), skillID, organizationID, true)
	resp.Diagnostics.Append(mappingDiags...)
	if resp.Diagnostics.HasError() {
		return
	}
	data = DataSourceModel{
		SkillID:     mapped.ID,
		ID:          mapped.ID,
		Name:        mapped.Name,
		Description: mapped.Description,
		Prompt:      mapped.Prompt,
		Command:     mapped.Command,
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
