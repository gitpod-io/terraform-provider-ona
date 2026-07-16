// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package runner

import (
	"context"
	"fmt"
	"strings"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdata"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/providerdiag"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	resourceschema "github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &SCMIntegrationResource{}
var _ resource.ResourceWithConfigure = &SCMIntegrationResource{}
var _ resource.ResourceWithImportState = &SCMIntegrationResource{}
var _ resource.ResourceWithValidateConfig = &SCMIntegrationResource{}

func NewSCMIntegrationResource() resource.Resource {
	return &SCMIntegrationResource{}
}

type SCMIntegrationResource struct {
	client *managementclient.ManagementPlane
}

const (
	scmAuthModeOAuth = "oauth"
	scmAuthModePAT   = "pat"

	scmIDAzureDevOpsEntra  = "azuredevops_entra"
	scmIDAzureDevOpsServer = "azuredevops_server"
)

type SCMIntegrationModel struct {
	ID                       types.String `tfsdk:"id"`
	RunnerID                 types.String `tfsdk:"runner_id"`
	SCMID                    types.String `tfsdk:"kind"`
	Host                     types.String `tfsdk:"host"`
	AuthMode                 types.String `tfsdk:"auth_mode"`
	OAuthClientID            types.String `tfsdk:"oauth_client_id"`
	OAuthClientSecret        types.String `tfsdk:"oauth_client_secret"`
	OAuthClientSecretVersion types.String `tfsdk:"oauth_client_secret_version"`
	IssuerURL                types.String `tfsdk:"issuer_url"`
	VirtualDirectory         types.String `tfsdk:"virtual_directory"`
}

func (r *SCMIntegrationResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_scm_integration"
}

func (r *SCMIntegrationResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		MarkdownDescription: "Ona runner SCM integration. Use this to configure how a runner authenticates to source control systems for projects assigned to that runner.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "SCM integration ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"runner_id": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Runner ID this SCM integration belongs to. Changing this value replaces the integration.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"kind": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "SCM integration kind. Use values such as `github`, `azuredevops_entra`, or `azuredevops_server`. Changing this value replaces the integration.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"host": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "SCM host name, for example `github.com` or an Azure DevOps Server host. Changing this value replaces the integration.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"auth_mode": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Authentication mode. Supported values are `oauth` and `pat`. Azure DevOps Server currently requires `pat`.",
			},
			"oauth_client_id": resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "OAuth app client ID. Required when `auth_mode` is `oauth`; omit when `auth_mode` is `pat`.",
			},
			"oauth_client_secret": resourceschema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				WriteOnly:           true,
				MarkdownDescription: "OAuth app client secret. This write-only value is sent to Ona but is not stored in Terraform plan or state. Required when creating an OAuth integration or rotating the OAuth secret.",
			},
			"oauth_client_secret_version": resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "User-managed version marker for resubmitting `oauth_client_secret` during rotation. Increment or otherwise change this value when supplying a new secret.",
			},
			"issuer_url": resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Issuer URL for Azure DevOps Entra ID OAuth integrations. Required when `kind` is `azuredevops_entra` and `auth_mode` is `oauth`.",
			},
			"virtual_directory": resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Virtual directory path for Azure DevOps Server integrations, such as `/tfs`. Required only when `kind` is `azuredevops_server`.",
			},
		},
	}
}

func (r *SCMIntegrationResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *SCMIntegrationResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var data SCMIntegrationModel
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("kind"), &data.SCMID)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("auth_mode"), &data.AuthMode)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("oauth_client_id"), &data.OAuthClientID)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("oauth_client_secret_version"), &data.OAuthClientSecretVersion)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("issuer_url"), &data.IssuerURL)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("virtual_directory"), &data.VirtualDirectory)...)
	if resp.Diagnostics.HasError() {
		return
	}

	validateSCMIntegrationModel(data, &resp.Diagnostics)
}

func (r *SCMIntegrationResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data SCMIntegrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	secret := readOAuthClientSecret(ctx, req.Config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before creating ona_scm_integration resources.",
		)
		return
	}

	createReq, diags := createSCMIntegrationRequest(data, secret)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	result, err := r.client.RunnerConfigurationService().CreateSCMIntegration(ctx, connect.NewRequest(createReq))
	if err != nil {
		addSCMIntegrationWriteError(&resp.Diagnostics, "Unable to Create Ona SCM Integration", data, err)
		return
	}

	data.ID = types.StringValue(result.Msg.GetId())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	integration, err := r.getSCMIntegration(ctx, result.Msg.GetId())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Created Ona SCM Integration", "reading the created Ona runner SCM integration", err)
		return
	}
	if integration == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	planned := data
	populateSCMIntegrationModel(&data, integration)
	preserveSCMIntegrationPlannedInputs(&data, planned)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SCMIntegrationResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data SCMIntegrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before reading ona_scm_integration resources.",
		)
		return
	}

	id := data.ID.ValueString()
	if id == "" {
		resp.Diagnostics.AddError("Unable to Read Ona SCM Integration", "SCM integration ID is empty.")
		return
	}

	integration, err := r.getSCMIntegration(ctx, id)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona SCM Integration", "reading the Ona runner SCM integration", err)
		return
	}
	if integration == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	prior := data
	data = SCMIntegrationModel{}
	populateSCMIntegrationModel(&data, integration)
	data.OAuthClientSecretVersion = prior.OAuthClientSecretVersion
	if data.SCMID.ValueString() == scmIDAzureDevOpsEntra && isPATSCMIntegration(data) {
		// issuer_url is a Terraform-only compatibility input for Azure DevOps Entra PAT.
		data.IssuerURL = prior.IssuerURL
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SCMIntegrationResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data SCMIntegrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var prior SCMIntegrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}

	secret := readOAuthClientSecret(ctx, req.Config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before updating ona_scm_integration resources.",
		)
		return
	}

	updateReq, diags := updateSCMIntegrationRequest(data, prior, secret)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if _, err := r.client.RunnerConfigurationService().UpdateSCMIntegration(ctx, connect.NewRequest(updateReq)); err != nil {
		addSCMIntegrationWriteError(&resp.Diagnostics, "Unable to Update Ona SCM Integration", data, err)
		return
	}

	integration, err := r.getSCMIntegration(ctx, data.ID.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Updated Ona SCM Integration", "reading the updated Ona runner SCM integration", err)
		return
	}
	if integration == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	planned := data
	populateSCMIntegrationModel(&data, integration)
	preserveSCMIntegrationPlannedInputs(&data, planned)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *SCMIntegrationResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data SCMIntegrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if r.client == nil {
		resp.Diagnostics.AddError(
			"Ona API Client Is Not Configured",
			"Set the provider token argument or ONA_TOKEN before deleting ona_scm_integration resources.",
		)
		return
	}

	id := data.ID.ValueString()
	if id == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	_, err := r.client.RunnerConfigurationService().DeleteSCMIntegration(ctx, connect.NewRequest(&v1.DeleteSCMIntegrationRequest{Id: id}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Delete Ona SCM Integration", "deleting the Ona runner SCM integration", err)
		return
	}

	resp.State.RemoveResource(ctx)
}

func (r *SCMIntegrationResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *SCMIntegrationResource) getSCMIntegration(ctx context.Context, id string) (*v1.SCMIntegration, error) {
	result, err := r.client.RunnerConfigurationService().GetSCMIntegration(ctx, connect.NewRequest(&v1.GetSCMIntegrationRequest{Id: id}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get SCM integration: %w", err)
	}
	return result.Msg.GetIntegration(), nil
}

func createSCMIntegrationRequest(data SCMIntegrationModel, secret types.String) (*v1.CreateSCMIntegrationRequest, diag.Diagnostics) {
	var diags diag.Diagnostics

	validateSCMIntegrationModel(data, &diags)
	if isOAuthSCMIntegration(data) && !isKnownString(secret) {
		diags.AddAttributeError(path.Root("oauth_client_secret"), "Missing OAuth Client Secret", "Set oauth_client_secret when auth_mode is \"oauth\".")
	}
	if isPATSCMIntegration(data) && isKnownString(secret) {
		diags.AddAttributeError(path.Root("oauth_client_secret"), "Unexpected OAuth Client Secret", "Do not set oauth_client_secret when auth_mode is \"pat\".")
	}
	if diags.HasError() {
		return nil, diags
	}

	req := &v1.CreateSCMIntegrationRequest{
		RunnerId: data.RunnerID.ValueString(),
		ScmId:    data.SCMID.ValueString(),
		Host:     data.Host.ValueString(),
		Pat:      isPATSCMIntegration(data),
	}
	if isOAuthSCMIntegration(data) {
		req.OauthClientId = ptr(data.OAuthClientID.ValueString())
		req.OauthPlaintextClientSecret = ptr(secret.ValueString())
	}
	if isOAuthSCMIntegration(data) && isKnownString(data.IssuerURL) {
		req.IssuerUrl = ptr(data.IssuerURL.ValueString())
	}
	if isKnownString(data.VirtualDirectory) {
		req.VirtualDirectory = ptr(data.VirtualDirectory.ValueString())
	}
	return req, diags
}

func updateSCMIntegrationRequest(data SCMIntegrationModel, prior SCMIntegrationModel, secret types.String) (*v1.UpdateSCMIntegrationRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	validateSCMIntegrationModel(data, &diags)
	if isPATSCMIntegration(data) && isKnownString(secret) {
		diags.AddAttributeError(path.Root("oauth_client_secret"), "Unexpected OAuth Client Secret", "Do not set oauth_client_secret when auth_mode is \"pat\".")
	}
	if diags.HasError() {
		return nil, diags
	}

	req := &v1.UpdateSCMIntegrationRequest{
		Id: data.ID.ValueString(),
	}
	if isKnownString(data.AuthMode) {
		req.Pat = ptr(isPATSCMIntegration(data))
	}
	if isOAuthSCMIntegration(data) && isKnownString(data.OAuthClientID) {
		req.OauthClientId = ptr(data.OAuthClientID.ValueString())
	} else if isPATSCMIntegration(data) || data.OAuthClientID.IsNull() && !prior.OAuthClientID.IsNull() {
		req.OauthClientId = ptr("")
	}
	if isOAuthSCMIntegration(data) && isKnownString(data.IssuerURL) {
		req.IssuerUrl = ptr(data.IssuerURL.ValueString())
	} else if !isPATSCMIntegration(data) && data.IssuerURL.IsNull() && !prior.IssuerURL.IsNull() {
		req.IssuerUrl = ptr("")
	}
	if isKnownString(data.VirtualDirectory) {
		req.VirtualDirectory = ptr(data.VirtualDirectory.ValueString())
	} else if data.VirtualDirectory.IsNull() && !prior.VirtualDirectory.IsNull() {
		req.VirtualDirectory = ptr("")
	}
	if oauthSecretRequiredForUpdate(data, prior) {
		if !isKnownString(secret) {
			diags.AddAttributeError(path.Root("oauth_client_secret"), "Missing OAuth Client Secret", "Set oauth_client_secret when enabling OAuth, changing oauth_client_id, or changing oauth_client_secret_version.")
			return nil, diags
		}
		req.OauthPlaintextClientSecret = ptr(secret.ValueString())
	}
	return req, diags
}

func populateSCMIntegrationModel(data *SCMIntegrationModel, integration *v1.SCMIntegration) {
	data.ID = types.StringValue(integration.GetId())
	data.RunnerID = types.StringValue(integration.GetRunnerId())
	data.SCMID = types.StringValue(integration.GetScmId())
	data.Host = types.StringValue(integration.GetHost())
	if oauth := integration.GetOauth(); oauth != nil {
		data.AuthMode = types.StringValue(scmAuthModeOAuth)
		data.OAuthClientID = stringOptionalValue(oauth.GetClientId())
		data.IssuerURL = stringOptionalValue(oauth.GetIssuerUrl())
	} else {
		data.AuthMode = types.StringNull()
		data.OAuthClientID = types.StringNull()
		data.IssuerURL = types.StringNull()
	}
	if integration.GetPat() {
		data.AuthMode = types.StringValue(scmAuthModePAT)
	}
	data.OAuthClientSecret = types.StringNull()
	data.OAuthClientSecretVersion = types.StringNull()
	data.VirtualDirectory = stringOptionalValue(integration.GetVirtualDirectory())
}

func preserveSCMIntegrationPlannedInputs(data *SCMIntegrationModel, planned SCMIntegrationModel) {
	data.RunnerID = preserveString(data.RunnerID, planned.RunnerID)
	data.SCMID = preserveString(data.SCMID, planned.SCMID)
	data.Host = preserveString(data.Host, planned.Host)
	data.AuthMode = preserveString(data.AuthMode, planned.AuthMode)
	data.OAuthClientID = preserveString(data.OAuthClientID, planned.OAuthClientID)
	data.OAuthClientSecret = types.StringNull()
	data.OAuthClientSecretVersion = preserveString(data.OAuthClientSecretVersion, planned.OAuthClientSecretVersion)
	data.IssuerURL = preserveString(data.IssuerURL, planned.IssuerURL)
	data.VirtualDirectory = preserveString(data.VirtualDirectory, planned.VirtualDirectory)
}

func readOAuthClientSecret(ctx context.Context, cfg tfsdk.Config, diags *diag.Diagnostics) types.String {
	var secret types.String
	diags.Append(cfg.GetAttribute(ctx, path.Root("oauth_client_secret"), &secret)...)
	return secret
}

func addSCMIntegrationWriteError(diags *diag.Diagnostics, summary string, data SCMIntegrationModel, err error) {
	if isOAuthSCMIntegration(data) && isRunnerPublicKeyMissingError(err) {
		diags.AddError(
			"Runner Public Key Is Not Available",
			fmt.Sprintf(
				"Ona cannot encrypt the OAuth client secret for runner %q because the runner has not registered its public key yet. Deploy the runner first, wait for it to register, then rerun this Terraform configuration.",
				data.RunnerID.ValueString(),
			),
		)
		return
	}

	providerdiag.AddAPIError(diags, summary, "writing the Ona runner SCM integration", err)
}

func isRunnerPublicKeyMissingError(err error) bool {
	return connect.CodeOf(err) == connect.CodeFailedPrecondition && strings.Contains(err.Error(), "runner does not have a public key")
}

func isKnownString(value types.String) bool {
	return !value.IsNull() && !value.IsUnknown() && value.ValueString() != ""
}

func isKnownBool(value types.Bool) bool {
	return !value.IsNull() && !value.IsUnknown()
}

func validateSCMIntegrationModel(data SCMIntegrationModel, diags *diag.Diagnostics) {
	if !isKnownString(data.AuthMode) {
		return
	}

	authMode := data.AuthMode.ValueString()
	switch authMode {
	case scmAuthModeOAuth:
		validateOAuthSCMIntegration(data, diags)
	case scmAuthModePAT:
		validatePATSCMIntegration(data, diags)
	default:
		diags.AddAttributeError(path.Root("auth_mode"), "Invalid SCM Authentication Mode", "Supported values are \"oauth\" and \"pat\".")
	}

	if !isKnownString(data.SCMID) {
		return
	}
	switch data.SCMID.ValueString() {
	case scmIDAzureDevOpsEntra:
		if authMode == scmAuthModeOAuth && !isKnownString(data.IssuerURL) {
			diags.AddAttributeError(path.Root("issuer_url"), "Missing Azure DevOps Entra Issuer URL", "Set issuer_url when kind is \"azuredevops_entra\".")
		}
		if isKnownString(data.VirtualDirectory) {
			diags.AddAttributeError(path.Root("virtual_directory"), "Unexpected Virtual Directory", "virtual_directory is only supported when kind is \"azuredevops_server\".")
		}
	case scmIDAzureDevOpsServer:
		if authMode != scmAuthModePAT {
			diags.AddAttributeError(path.Root("auth_mode"), "Invalid Azure DevOps Server Authentication Mode", "Azure DevOps Server SCM integrations currently require auth_mode=\"pat\".")
		}
		if !isKnownString(data.VirtualDirectory) {
			diags.AddAttributeError(path.Root("virtual_directory"), "Missing Azure DevOps Server Virtual Directory", "Set virtual_directory when kind is \"azuredevops_server\".")
		}
		if isKnownString(data.IssuerURL) {
			diags.AddAttributeError(path.Root("issuer_url"), "Unexpected Issuer URL", "issuer_url is only accepted when kind is \"azuredevops_entra\".")
		}
	default:
		if isKnownString(data.IssuerURL) {
			diags.AddAttributeError(path.Root("issuer_url"), "Unexpected Issuer URL", "issuer_url is only accepted when kind is \"azuredevops_entra\".")
		}
		if isKnownString(data.VirtualDirectory) {
			diags.AddAttributeError(path.Root("virtual_directory"), "Unexpected Virtual Directory", "virtual_directory is only supported when kind is \"azuredevops_server\".")
		}
	}
}

func validateOAuthSCMIntegration(data SCMIntegrationModel, diags *diag.Diagnostics) {
	if !isKnownString(data.OAuthClientID) {
		diags.AddAttributeError(path.Root("oauth_client_id"), "Missing OAuth Client ID", "Set oauth_client_id when auth_mode is \"oauth\".")
	}
}

func validatePATSCMIntegration(data SCMIntegrationModel, diags *diag.Diagnostics) {
	if isKnownString(data.OAuthClientID) {
		diags.AddAttributeError(path.Root("oauth_client_id"), "Unexpected OAuth Client ID", "Do not set oauth_client_id when auth_mode is \"pat\".")
	}
	if !data.OAuthClientSecretVersion.IsNull() && !data.OAuthClientSecretVersion.IsUnknown() {
		diags.AddAttributeError(path.Root("oauth_client_secret_version"), "Unexpected OAuth Client Secret Version", "Do not set oauth_client_secret_version when auth_mode is \"pat\".")
	}
}

func isOAuthSCMIntegration(data SCMIntegrationModel) bool {
	return data.AuthMode.ValueString() == scmAuthModeOAuth
}

func isPATSCMIntegration(data SCMIntegrationModel) bool {
	return data.AuthMode.ValueString() == scmAuthModePAT
}

func oauthSecretRequiredForUpdate(data SCMIntegrationModel, prior SCMIntegrationModel) bool {
	if !isOAuthSCMIntegration(data) {
		return false
	}
	return prior.AuthMode.ValueString() != scmAuthModeOAuth ||
		stringValueChanged(data.OAuthClientID, prior.OAuthClientID) ||
		secretVersionChanged(data.OAuthClientSecretVersion, prior.OAuthClientSecretVersion)
}

func stringValueChanged(current types.String, prior types.String) bool {
	if current.IsUnknown() || prior.IsUnknown() {
		return false
	}
	if current.IsNull() && prior.IsNull() {
		return false
	}
	if current.IsNull() != prior.IsNull() {
		return true
	}
	return current.ValueString() != prior.ValueString()
}

func secretVersionChanged(current types.String, prior types.String) bool {
	if current.IsUnknown() || prior.IsUnknown() {
		return false
	}
	if current.IsNull() && prior.IsNull() {
		return false
	}
	if current.IsNull() != prior.IsNull() {
		return true
	}
	return current.ValueString() != prior.ValueString()
}
