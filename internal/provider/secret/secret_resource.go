// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package secret

import (
	"context"
	"fmt"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &Resource{}
var _ resource.ResourceWithConfigure = &Resource{}
var _ resource.ResourceWithIdentity = &Resource{}
var _ resource.ResourceWithImportState = &Resource{}
var _ resource.ResourceWithValidateConfig = &Resource{}

func NewResource() resource.Resource {
	return &Resource{}
}

type Resource struct {
	client *managementclient.ManagementPlane
}

func (r *Resource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_secret"
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
	var data Model
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("scope"), &data.Scope)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("project_id"), &data.ProjectID)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("user_id"), &data.UserID)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("service_account_id"), &data.ServiceAccountID)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("name"), &data.Name)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("environment_variable"), &data.EnvironmentVariable)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("file_path"), &data.FilePath)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("container_registry_basic_auth_host"), &data.ContainerRegistryBasicAuthHost)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("api_only"), &data.APIOnly)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("credential_proxy"), &data.CredentialProxy)...)
	if resp.Diagnostics.HasError() {
		return
	}
	validateModel(ctx, data, false, &resp.Diagnostics)
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	value := readSecretValue(ctx, req.Config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.requireClient(&resp.Diagnostics, "creating") {
		return
	}

	scope := r.resolveScope(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	createReq, diags := createSecretRequest(ctx, data, value, scope)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.SecretService().CreateSecret(ctx, connect.NewRequest(createReq))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Create Ona Secret", "creating the Ona secret", err)
		return
	}
	if result.Msg.GetSecret().GetId() == "" {
		resp.Diagnostics.AddError("Unable to Create Ona Secret", "The Ona API returned a created secret without an ID.")
		return
	}

	data.ID = types.StringValue(result.Msg.GetSecret().GetId())
	data.ProjectID = scope.ProjectID
	data.UserID = scope.UserID
	data.ServiceAccountID = scope.ServiceAccountID
	data.Value = types.StringNull()
	resp.Diagnostics.Append(resp.Identity.Set(ctx, identityFromModel(data, scope.Scope.GetOrganizationId()))...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	planned := data
	populateModelFromSecret(ctx, &data, result.Msg.GetSecret(), &resp.Diagnostics)
	preservePlannedInputs(&data, planned)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data Model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.requireClient(&resp.Diagnostics, "reading") {
		return
	}
	if data.ID.ValueString() == "" {
		resp.Diagnostics.AddError("Unable to Read Ona Secret", "Secret ID is empty.")
		return
	}

	scope := r.resolveScope(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	secret, err := r.findSecret(ctx, scope.Scope, data.ID.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Secret", "reading the Ona secret", err)
		return
	}
	if secret == nil {
		// ListSecrets is the only metadata read endpoint here; a missing list row
		// is not a definitive NotFound for this secret ID.
		resp.Diagnostics.Append(resp.Identity.Set(ctx, identityFromModel(data, scope.Scope.GetOrganizationId()))...)
		resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
		return
	}

	prior := data
	data = Model{}
	populateModelFromSecret(ctx, &data, secret, &resp.Diagnostics)
	preserveTerraformOnlyState(&data, prior)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, identityFromModel(data, scope.Scope.GetOrganizationId()))...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var prior Model
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.requireClient(&resp.Diagnostics, "updating") {
		return
	}

	scope := r.resolveScope(ctx, &data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if stringValueChanged(data.ValueVersion, prior.ValueVersion) {
		value := readSecretValue(ctx, req.Config, &resp.Diagnostics)
		if resp.Diagnostics.HasError() {
			return
		}
		if !isKnownString(value) {
			resp.Diagnostics.AddAttributeError(path.Root("value"), "Missing Secret Value", "Set value when changing value_version.")
			return
		}
		if _, err := r.client.SecretService().UpdateSecretValue(ctx, connect.NewRequest(&v1.UpdateSecretValueRequest{
			SecretId: data.ID.ValueString(),
			Value:    value.ValueString(),
		})); err != nil {
			providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Update Ona Secret Value", "updating the Ona secret value", err)
			return
		}
	}

	secret, err := r.findSecret(ctx, scope.Scope, data.ID.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Updated Ona Secret", "reading the updated Ona secret", err)
		return
	}
	if secret == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	planned := data
	populateModelFromSecret(ctx, &data, secret, &resp.Diagnostics)
	preservePlannedInputs(&data, planned)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, identityFromModel(data, scope.Scope.GetOrganizationId()))...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data Model
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.requireClient(&resp.Diagnostics, "deleting") {
		return
	}
	if data.ID.ValueString() == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	_, err := r.client.SecretService().DeleteSecret(ctx, connect.NewRequest(&v1.DeleteSecretRequest{SecretId: data.ID.ValueString()}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Delete Ona Secret", "deleting the Ona secret", err)
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if req.ID == "" {
		var identity IdentityModel
		resp.Diagnostics.Append(req.Identity.Get(ctx, &identity)...)
		if resp.Diagnostics.HasError() {
			return
		}
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), identity.ID)...)
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("scope"), identity.Scope)...)
		switch identity.Scope.ValueString() {
		case scopeOrganization:
			if !isKnownString(identity.OrganizationID) {
				resp.Diagnostics.AddError("Invalid Secret Identity", "organization_id is required for organization scope.")
				return
			}
		case scopeProject:
			if !isKnownString(identity.ProjectID) {
				resp.Diagnostics.AddError("Invalid Secret Identity", "project_id is required for project scope.")
				return
			}
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), identity.ProjectID)...)
		case scopeUser:
			if !isKnownString(identity.UserID) {
				resp.Diagnostics.AddError("Invalid Secret Identity", "user_id is required for user scope.")
				return
			}
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_id"), identity.UserID)...)
		case scopeServiceAccount:
			if !isKnownString(identity.ServiceAccountID) {
				resp.Diagnostics.AddError("Invalid Secret Identity", "service_account_id is required for service_account scope.")
				return
			}
			resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("service_account_id"), identity.ServiceAccountID)...)
		default:
			resp.Diagnostics.AddError("Invalid Secret Identity", "scope must be organization, project, user, or service_account.")
		}
		resp.Diagnostics.Append(resp.Identity.Set(ctx, identity)...)
		return
	}
	importState, diags := parseImportID(req.ID)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), types.StringValue(importState.ID))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("scope"), types.StringValue(importState.Scope))...)
	if importState.ProjectID != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project_id"), types.StringValue(importState.ProjectID))...)
	}
	if importState.UserID != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_id"), types.StringValue(importState.UserID))...)
	}
	if importState.ServiceAccountID != "" {
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("service_account_id"), types.StringValue(importState.ServiceAccountID))...)
	}
	identity := IdentityModel{
		ID:               types.StringValue(importState.ID),
		Scope:            types.StringValue(importState.Scope),
		ProjectID:        types.StringNull(),
		UserID:           types.StringNull(),
		ServiceAccountID: types.StringNull(),
		OrganizationID:   types.StringNull(),
	}
	switch importState.Scope {
	case scopeOrganization:
		authenticated, err := r.authenticatedIdentity(ctx)
		if err != nil {
			providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Ona Organization", "reading the authenticated Ona identity during import", err)
			return
		}
		identity.OrganizationID = types.StringValue(authenticated.GetOrganizationId())
	case scopeProject:
		identity.ProjectID = types.StringValue(importState.ProjectID)
	case scopeUser:
		identity.UserID = types.StringValue(importState.UserID)
	case scopeServiceAccount:
		identity.ServiceAccountID = types.StringValue(importState.ServiceAccountID)
	}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, identity)...)
}

func (r *Resource) requireClient(diags *diag.Diagnostics, action string) bool {
	if r.client != nil {
		return true
	}
	diags.AddError(
		"Ona API Client Is Not Configured",
		fmt.Sprintf("Set the provider token argument or ONA_TOKEN before %s ona_secret resources.", action),
	)
	return false
}

func (r *Resource) resolveScope(ctx context.Context, data *Model, diags *diag.Diagnostics) resolvedScope {
	validateScope(*data, true, diags)
	if diags.HasError() {
		return resolvedScope{}
	}

	identity, err := r.authenticatedIdentity(ctx)
	if err != nil {
		providerdiag.AddAPIError(diags, "Unable to Resolve Ona Organization", "reading the authenticated Ona identity", err)
		return resolvedScope{}
	}
	if identity.GetOrganizationId() == "" {
		diags.AddError("Unable to Resolve Ona Organization", "The authenticated Ona identity did not include an organization ID.")
		return resolvedScope{}
	}
	organizationID := types.StringValue(identity.GetOrganizationId())

	if data.Scope.ValueString() == scopeUser {
		if !isKnownString(data.UserID) {
			if identity.GetSubject().GetPrincipal() != v1.Principal_PRINCIPAL_USER || identity.GetSubject().GetId() == "" {
				diags.AddAttributeError(path.Root("user_id"), "Missing User ID", "Set user_id when scope is \"user\" unless the provider is authenticated as a user.")
				return resolvedScope{}
			}
			data.UserID = types.StringValue(identity.GetSubject().GetId())
		}
	}

	scope, scopeDiags := secretScopeFromModel(*data, organizationID)
	diags.Append(scopeDiags...)
	if diags.HasError() {
		return resolvedScope{}
	}
	return resolvedScope{
		Scope:            scope,
		ProjectID:        data.ProjectID,
		UserID:           data.UserID,
		ServiceAccountID: data.ServiceAccountID,
	}
}

func (r *Resource) authenticatedIdentity(ctx context.Context) (*v1.GetAuthenticatedIdentityResponse, error) {
	result, err := r.client.IdentityService().GetAuthenticatedIdentity(ctx, connect.NewRequest(&v1.GetAuthenticatedIdentityRequest{}))
	if err != nil {
		return nil, fmt.Errorf("get authenticated identity: %w", err)
	}
	return result.Msg, nil
}

func (r *Resource) findSecret(ctx context.Context, scope *v1.SecretScope, id string) (*v1.Secret, error) {
	token := ""
	for {
		result, err := r.client.SecretService().ListSecrets(ctx, connect.NewRequest(&v1.ListSecretsRequest{
			Pagination: &v1.PaginationRequest{
				PageSize: 100,
				Token:    token,
			},
			Filter: &v1.ListSecretsRequest_Filter{
				Scope: scope,
			},
		}))
		if err != nil {
			return nil, fmt.Errorf("list secrets: %w", err)
		}
		for _, secret := range result.Msg.GetSecrets() {
			if secret.GetId() == id {
				return secret, nil
			}
		}
		token = result.Msg.GetPagination().GetNextToken()
		if token == "" {
			return nil, nil
		}
	}
}

func readSecretValue(ctx context.Context, cfg tfsdk.Config, diags *diag.Diagnostics) types.String {
	var value types.String
	diags.Append(cfg.GetAttribute(ctx, path.Root("value"), &value)...)
	return value
}
