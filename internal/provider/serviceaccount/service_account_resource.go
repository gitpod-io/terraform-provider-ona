// Copyright IBM Corp. 2021, 2026
// SPDX-License-Identifier: MPL-2.0

package serviceaccount

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/api/go/client"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/types/known/timestamppb"
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

type Model struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	ValidUntil  types.String `tfsdk:"valid_until"`
	CreatedAt   types.String `tfsdk:"created_at"`
	Creator     types.Object `tfsdk:"creator"`
}

func (r *Resource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_service_account"
}

func (r *Resource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		MarkdownDescription: "Ona service account for non-human API access. Use service accounts for Terraform automation and issue tokens with the `ona_service_account_token` ephemeral resource.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Service account ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Service account display name shown in Ona.",
			},
			"description": resourceschema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Service account description. Omit to leave the description empty.",
			},
			"valid_until": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "RFC3339 timestamp when this service account expires, for example `2030-01-02T03:04:05Z`. Changing this value replaces the service account.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"created_at": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Time when the service account was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"creator": resourceschema.SingleNestedAttribute{
				Computed:            true,
				MarkdownDescription: "Identity that created the service account.",
				Attributes: map[string]resourceschema.Attribute{
					"id": resourceschema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Creator subject ID.",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
					"principal": resourceschema.StringAttribute{
						Computed:            true,
						MarkdownDescription: "Creator principal type.",
						PlanModifiers: []planmodifier.String{
							stringplanmodifier.UseStateForUnknown(),
						},
					},
				},
			},
		},
	}
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
	var validUntil types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("valid_until"), &validUntil)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateServiceAccountValidUntil(validUntil, time.Now().UTC(), &resp.Diagnostics)
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before creating ona_service_account resources.",
		)
		return
	}

	createReq, diags := createServiceAccountRequest(data)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.ServiceAccountService().CreateServiceAccount(ctx, connect.NewRequest(createReq))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Create Ona Service Account", "creating the Ona service account", err)
		return
	}
	if result.Msg.GetServiceAccount() == nil {
		resp.Diagnostics.AddError("Unable to Create Ona Service Account", "The Ona API returned an empty service account.")
		return
	}

	planned := data
	populateModelFromServiceAccount(&data, result.Msg.GetServiceAccount())
	preservePlannedInputs(&data, planned)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data Model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before reading ona_service_account resources.",
		)
		return
	}

	id := serviceAccountID(data)
	if id == "" {
		resp.Diagnostics.AddError("Unable to Read Ona Service Account", "Service account ID is empty.")
		return
	}

	account, err := r.getServiceAccount(ctx, id)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Service Account", "reading the Ona service account", err)
		return
	}
	if account == nil || account.GetSuspended() {
		resp.State.RemoveResource(ctx)
		return
	}

	data = Model{}
	populateModelFromServiceAccount(&data, account)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before updating ona_service_account resources.",
		)
		return
	}

	id := serviceAccountID(data)
	if id == "" {
		resp.Diagnostics.AddError("Unable to Update Ona Service Account", "Service account ID is empty.")
		return
	}

	updateReq := &v1.UpdateServiceAccountRequest{
		ServiceAccountId: id,
	}
	if !data.Name.IsNull() && !data.Name.IsUnknown() {
		name := data.Name.ValueString()
		updateReq.Name = &name
	}
	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		description := data.Description.ValueString()
		updateReq.Description = &description
	}

	result, err := r.client.ServiceAccountService().UpdateServiceAccount(ctx, connect.NewRequest(updateReq))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Update Ona Service Account", "updating the Ona service account", err)
		return
	}
	if result.Msg.GetServiceAccount() == nil {
		resp.Diagnostics.AddError("Unable to Update Ona Service Account", "The Ona API returned an empty service account.")
		return
	}

	planned := data
	populateModelFromServiceAccount(&data, result.Msg.GetServiceAccount())
	preservePlannedInputs(&data, planned)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data Model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before deleting ona_service_account resources.",
		)
		return
	}

	id := serviceAccountID(data)
	if id == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	_, err := r.client.ServiceAccountService().DeleteServiceAccount(ctx, connect.NewRequest(&v1.DeleteServiceAccountRequest{
		ServiceAccountId: id,
	}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Delete Ona Service Account", "deleting the Ona service account", err)
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *Resource) getServiceAccount(ctx context.Context, id string) (*v1.ServiceAccount, error) {
	result, err := r.client.ServiceAccountService().GetServiceAccount(ctx, connect.NewRequest(&v1.GetServiceAccountRequest{
		ServiceAccountId: id,
	}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get service account: %w", err)
	}
	return result.Msg.GetServiceAccount(), nil
}

func createServiceAccountRequest(data Model) (*v1.CreateServiceAccountRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	validUntil, err := timestampFromRFC3339(data.ValidUntil)
	if err != nil {
		diags.AddAttributeError(path.Root("valid_until"), "Invalid Service Account Expiration", err.Error())
		return nil, diags
	}

	req := &v1.CreateServiceAccountRequest{
		Name:       data.Name.ValueString(),
		ValidUntil: validUntil,
	}
	if !data.Description.IsNull() && !data.Description.IsUnknown() {
		req.Description = data.Description.ValueString()
	}
	return req, diags
}

func populateModelFromServiceAccount(data *Model, account *v1.ServiceAccount) {
	id := account.GetId()
	data.ID = types.StringValue(id)
	data.Name = types.StringValue(account.GetName())
	data.Description = types.StringValue(account.GetDescription())
	data.ValidUntil = timestampValue(account.GetValidUntil())
	data.CreatedAt = timestampValue(account.GetCreatedAt())
	data.Creator = creatorModel(account.GetCreator())
}

func preservePlannedInputs(data *Model, planned Model) {
	data.Name = preserveString(data.Name, planned.Name)
	data.Description = preserveString(data.Description, planned.Description)
	data.ValidUntil = preserveString(data.ValidUntil, planned.ValidUntil)
}

func preserveString(current types.String, planned types.String) types.String {
	if planned.IsNull() || planned.IsUnknown() {
		return current
	}
	return planned
}

func serviceAccountID(data Model) string {
	if !data.ID.IsNull() && !data.ID.IsUnknown() {
		return data.ID.ValueString()
	}
	return ""
}

func timestampFromRFC3339(value types.String) (*timestamppb.Timestamp, error) {
	parsed, err := serviceAccountValidUntilTime(value, time.Now().UTC())
	if err != nil {
		return nil, err
	}
	return timestamppb.New(parsed), nil
}

func validateServiceAccountValidUntil(value types.String, now time.Time, diags *diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return
	}
	if _, err := serviceAccountValidUntilTime(value, now); err != nil {
		diags.AddAttributeError(path.Root("valid_until"), "Invalid Service Account Expiration", err.Error())
	}
}

func serviceAccountValidUntilTime(value types.String, now time.Time) (time.Time, error) {
	if value.IsNull() || value.IsUnknown() {
		return time.Time{}, fmt.Errorf("valid_until must be known")
	}
	parsed, err := time.Parse(time.RFC3339, value.ValueString())
	if err != nil {
		return time.Time{}, fmt.Errorf("valid_until must be an RFC3339 timestamp: %w", err)
	}
	if !parsed.After(now) {
		return time.Time{}, fmt.Errorf("valid_until must be in the future")
	}
	return parsed, nil
}

func timestampValue(value *timestamppb.Timestamp) types.String {
	if value == nil {
		return types.StringNull()
	}
	return types.StringValue(value.AsTime().UTC().Format(time.RFC3339))
}

func stringOptionalValue(value string) types.String {
	if value == "" {
		return types.StringNull()
	}
	return types.StringValue(value)
}

func creatorAttributeTypes() map[string]attr.Type {
	return map[string]attr.Type{
		"id":        types.StringType,
		"principal": types.StringType,
	}
}

func creatorModel(creator *v1.Subject) types.Object {
	if creator == nil {
		return types.ObjectNull(creatorAttributeTypes())
	}
	return types.ObjectValueMust(creatorAttributeTypes(), map[string]attr.Value{
		"id":        stringOptionalValue(creator.GetId()),
		"principal": types.StringValue(principalToString(creator.GetPrincipal())),
	})
}

func principalToString(principal v1.Principal) string {
	switch principal {
	case v1.Principal_PRINCIPAL_USER:
		return "user"
	case v1.Principal_PRINCIPAL_SERVICE_ACCOUNT:
		return "service_account"
	case v1.Principal_PRINCIPAL_RUNNER:
		return "runner"
	default:
		return "unspecified"
	}
}
