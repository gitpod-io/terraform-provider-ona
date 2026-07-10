// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package runner

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/api/go/client"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &SingularDataSource{}
var _ datasource.DataSourceWithConfigure = &SingularDataSource{}

func NewSingularDataSource() datasource.DataSource {
	return &SingularDataSource{}
}

type SingularDataSource struct {
	client *managementclient.ManagementPlane
}

type DataSourceModel struct {
	ID                        types.String        `tfsdk:"id"`
	RunnerID                  types.String        `tfsdk:"runner_id"`
	Name                      types.String        `tfsdk:"name"`
	RunnerProvider            types.String        `tfsdk:"runner_provider"`
	Kind                      types.String        `tfsdk:"kind"`
	CloudFormationTemplateURL types.String        `tfsdk:"cloudformation_template_url"`
	CreatedAt                 types.String        `tfsdk:"created_at"`
	UpdatedAt                 types.String        `tfsdk:"updated_at"`
	Configuration             *ConfigurationModel `tfsdk:"configuration"`
	Status                    *StatusModel        `tfsdk:"status"`
	Creator                   *CreatorModel       `tfsdk:"creator"`
}

func (d *SingularDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_runner"
}

func (d *SingularDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = singularDataSourceSchema()
}

func (d *SingularDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *SingularDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data DataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if d.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before reading ona_runner data sources.",
		)
		return
	}

	id := dataSourceRunnerID(data)
	if id == "" {
		resp.Diagnostics.AddAttributeError(
			path.Root("runner_id"),
			"Missing Runner ID",
			"Set runner_id before reading an Ona runner data source.",
		)
		return
	}

	result, err := d.client.RunnerService().GetRunner(ctx, connect.NewRequest(&v1.GetRunnerRequest{
		RunnerId: id,
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			resp.Diagnostics.AddAttributeError(
				path.Root("runner_id"),
				"Runner Not Found",
				fmt.Sprintf("No Ona runner exists with runner_id %q.", id),
			)
			return
		}
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Runner", "reading the Ona runner data source", err)
		return
	}
	if result.Msg.GetRunner() == nil {
		resp.Diagnostics.AddError("Unable to Read Ona Runner", "The Ona API returned an empty runner.")
		return
	}

	data = DataSourceModel{}
	populateDataSourceModelFromRunner(&data, result.Msg.GetRunner())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func singularDataSourceSchema() datasourceschema.Schema {
	attributes := dataSourceRunnerAttributes(datasourceschema.StringAttribute{
		Required:            true,
		MarkdownDescription: "Ona runner ID to look up.",
	})

	return datasourceschema.Schema{
		MarkdownDescription: "Fetches an Ona runner by ID and exposes the same computed fields as the `ona_runner` resource.",
		Attributes:          attributes,
	}
}

func dataSourceRunnerAttributes(runnerID datasourceschema.StringAttribute) map[string]datasourceschema.Attribute {
	return map[string]datasourceschema.Attribute{
		"id": datasourceschema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Terraform data source ID. This is the same value as `runner_id`.",
		},
		"runner_id": runnerID,
		"name": datasourceschema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Runner display name.",
		},
		"runner_provider": datasourceschema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Runner cloud provider, such as `aws_ec2` or `gcp`.",
		},
		"kind": datasourceschema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Runner kind deduced by the API from the provider.",
		},
		"cloudformation_template_url": datasourceschema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "CloudFormation template URL for AWS EC2 runner setup. This is populated only for `aws_ec2` runners and is null for GCP runners.",
		},
		"created_at": datasourceschema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Time when the runner was created.",
		},
		"updated_at": datasourceschema.StringAttribute{
			Computed:            true,
			MarkdownDescription: "Time when the runner was last updated.",
		},
		"configuration": dataSourceConfigurationSchema(),
		"status":        dataSourceStatusSchema(),
		"creator":       dataSourceCreatorSchema(),
	}
}

func dataSourceConfigurationSchema() datasourceschema.SingleNestedAttribute {
	return datasourceschema.SingleNestedAttribute{
		Computed:            true,
		MarkdownDescription: "Runner configuration.",
		Attributes: map[string]datasourceschema.Attribute{
			"region": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Cloud region configured for the runner, when the runner provider uses a region.",
			},
			"release_channel": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Runner release channel, such as `stable` or `latest`.",
			},
			"auto_update": datasourceschema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the runner automatically updates itself.",
			},
			"devcontainer_image_cache_enabled": datasourceschema.BoolAttribute{
				Computed:            true,
				MarkdownDescription: "Whether the shared devcontainer build cache is enabled for this runner.",
			},
			"log_level": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Runner log level, such as `debug`, `info`, `warn`, or `error`.",
			},
			"update_window": datasourceschema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Daily UTC window during which auto-updates may run.",
				Attributes: map[string]datasourceschema.Attribute{
					"start": datasourceschema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Start time in `HH:00` UTC format.",
					},
					"end": datasourceschema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "End time in `HH:00` UTC format.",
					},
				},
			},
		},
	}
}

func populateDataSourceModelFromRunner(data *DataSourceModel, runner *v1.Runner) {
	var model RunnerModel
	populateModelFromRunner(&model, runner)

	data.ID = model.ID
	data.RunnerID = model.RunnerID
	data.Name = model.Name
	data.RunnerProvider = model.RunnerProvider
	data.Kind = model.Kind
	data.CloudFormationTemplateURL = model.CloudFormationTemplateURL
	data.CreatedAt = model.CreatedAt
	data.UpdatedAt = model.UpdatedAt
	data.Configuration = model.Configuration
	data.Status = model.Status
	data.Creator = model.Creator
}

func dataSourceRunnerID(data DataSourceModel) string {
	if !data.RunnerID.IsNull() && !data.RunnerID.IsUnknown() && data.RunnerID.ValueString() != "" {
		return data.RunnerID.ValueString()
	}
	if !data.ID.IsNull() && !data.ID.IsUnknown() {
		return data.ID.ValueString()
	}
	return ""
}

func dataSourceStatusSchema() datasourceschema.SingleNestedAttribute {
	return datasourceschema.SingleNestedAttribute{
		Computed:            true,
		MarkdownDescription: "Runner status reported by the runner.",
		Attributes: map[string]datasourceschema.Attribute{
			"phase": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Runner phase.",
			},
			"region": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Actual region reported by the runner.",
			},
			"message": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Runner status message.",
			},
			"version": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Runner version.",
			},
			"log_url": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Runner log URL.",
			},
			"updated_at": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the runner status was last updated.",
			},
			"system_details": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Runner system details.",
			},
			"support_bundle_url": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Runner support bundle URL.",
			},
		},
	}
}

func dataSourceCreatorSchema() datasourceschema.SingleNestedAttribute {
	return datasourceschema.SingleNestedAttribute{
		Computed:            true,
		MarkdownDescription: "Identity that created the runner.",
		Attributes: map[string]datasourceschema.Attribute{
			"id": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Creator subject ID.",
			},
			"principal": datasourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Creator principal type.",
			},
		},
	}
}
