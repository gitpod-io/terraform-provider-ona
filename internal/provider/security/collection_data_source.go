// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package security

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/api/go/client"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	datasourceschema "github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSource = &PolicyCollectionDataSource{}
var _ datasource.DataSourceWithConfigure = &PolicyCollectionDataSource{}

func NewPolicyCollectionDataSource() datasource.DataSource {
	return &PolicyCollectionDataSource{}
}

type PolicyCollectionDataSource struct {
	client *managementclient.ManagementPlane
}

type PolicyCollectionModel struct {
	OrganizationID    types.String         `tfsdk:"organization_id"`
	Search            types.String         `tfsdk:"search"`
	SecurityPolicyIDs types.Set            `tfsdk:"security_policy_ids"`
	Policies          []PolicySummaryModel `tfsdk:"policies"`
}

type PolicySummaryModel struct {
	ID             types.String `tfsdk:"id"`
	OrganizationID types.String `tfsdk:"organization_id"`
	Name           types.String `tfsdk:"name"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func (d *PolicyCollectionDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_security_policies"
}

func (d *PolicyCollectionDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = datasourceschema.Schema{
		MarkdownDescription: "Lists Ona security policies for an organization.",
		Attributes: map[string]datasourceschema.Attribute{
			"organization_id": datasourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Organization ID to list security policies for.",
			},
			"search": datasourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Search string for filtering security policies by name.",
			},
			"security_policy_ids": datasourceschema.SetAttribute{
				Optional:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Security policy IDs to include. The API accepts at most 25 IDs.",
			},
			"policies": datasourceschema.ListNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Matching security policy summaries.",
				NestedObject: datasourceschema.NestedAttributeObject{
					Attributes: map[string]datasourceschema.Attribute{
						"id": datasourceschema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Security policy ID.",
						},
						"organization_id": datasourceschema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Organization ID that owns the security policy.",
						},
						"name": datasourceschema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Security policy name.",
						},
						"created_at": datasourceschema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Time when the security policy was created.",
						},
						"updated_at": datasourceschema.StringAttribute{
							Computed:            true,
							MarkdownDescription: "Time when the security policy was last updated.",
						},
					},
				},
			},
		},
	}
}

func (d *PolicyCollectionDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	api, ok := req.ProviderData.(*managementclient.ManagementPlane)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *client.ManagementPlane, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = api
}

func (d *PolicyCollectionDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data PolicyCollectionModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if d.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before reading ona_security_policies data sources.",
		)
		return
	}

	ids, diags := stringSliceFromSet(ctx, data.SecurityPolicyIDs)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	if len(ids) > 25 {
		resp.Diagnostics.AddAttributeError(
			pathRoot("security_policy_ids"),
			"Too Many Security Policy IDs",
			"The Ona API accepts at most 25 security_policy_ids.",
		)
		return
	}

	policies, err := d.listPolicies(ctx, data.OrganizationID.ValueString(), data.Search.ValueString(), ids)
	if err != nil {
		resp.Diagnostics.AddError("Unable to List Ona Security Policies", err.Error())
		return
	}

	data.Policies = make([]PolicySummaryModel, 0, len(policies))
	for _, policy := range policies {
		data.Policies = append(data.Policies, PolicySummaryModel{
			ID:             types.StringValue(policy.GetId()),
			OrganizationID: types.StringValue(policy.GetOrganizationId()),
			Name:           types.StringValue(policy.GetMetadata().GetName()),
			CreatedAt:      timestampValue(policy.GetCreatedAt()),
			UpdatedAt:      timestampValue(policy.GetUpdatedAt()),
		})
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *PolicyCollectionDataSource) listPolicies(ctx context.Context, organizationID string, search string, ids []string) ([]*v1.SecurityPolicy, error) {
	var result []*v1.SecurityPolicy
	var token string
	for {
		response, err := d.client.SecurityService().ListSecurityPolicies(ctx, connect.NewRequest(&v1.ListSecurityPoliciesRequest{
			Pagination: &v1.PaginationRequest{
				PageSize: 100,
				Token:    token,
			},
			Filter: &v1.ListSecurityPoliciesRequest_Filter{
				OrganizationId:    organizationID,
				SecurityPolicyIds: ids,
				Search:            search,
			},
		}))
		if err != nil {
			return nil, fmt.Errorf("list security policies: %w", err)
		}
		result = append(result, response.Msg.GetSecurityPolicies()...)
		token = response.Msg.GetPagination().GetNextToken()
		if token == "" {
			return result, nil
		}
	}
}

func stringSliceFromSet(ctx context.Context, set types.Set) ([]string, diag.Diagnostics) {
	var diags diag.Diagnostics
	if set.IsNull() || set.IsUnknown() {
		return nil, diags
	}
	var values []string
	diags.Append(set.ElementsAs(ctx, &values, false)...)
	return values, diags
}

func pathRoot(name string) path.Path {
	return path.Root(name)
}
