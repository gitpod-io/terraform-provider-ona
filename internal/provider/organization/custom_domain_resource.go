// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package organization

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ resource.Resource = &CustomDomainResource{}
var _ resource.ResourceWithConfigure = &CustomDomainResource{}
var _ resource.ResourceWithImportState = &CustomDomainResource{}
var _ resource.ResourceWithValidateConfig = &CustomDomainResource{}

const (
	customDomainPrivateOrganizationIDKey = "authenticated_organization_id"
	customDomainProviderAWS              = "aws"
	customDomainProviderGCP              = "gcp"
	customDomainImportCurrent            = "current"
)

var (
	awsCloudAccountIDPattern = regexp.MustCompile(`^[0-9]{12}$`)
	gcpCloudAccountIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{4,28}[a-z0-9]$`)
	customDomainNamePattern  = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?(?:\.[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?)*$`)
)

type privateState interface {
	GetKey(context.Context, string) ([]byte, diag.Diagnostics)
	SetKey(context.Context, string, []byte) diag.Diagnostics
}

func NewCustomDomainResource() resource.Resource {
	return &CustomDomainResource{}
}

type CustomDomainResource struct {
	client *managementclient.ManagementPlane
}

type CustomDomainModel struct {
	ID             types.String `tfsdk:"id"`
	DomainName     types.String `tfsdk:"domain_name"`
	CloudProvider  types.String `tfsdk:"cloud_provider"`
	CloudAccountID types.String `tfsdk:"cloud_account_id"`
	CreatedAt      types.String `tfsdk:"created_at"`
	UpdatedAt      types.String `tfsdk:"updated_at"`
}

func (r *CustomDomainResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_custom_domain"
}

func (r *CustomDomainResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		MarkdownDescription: "Ona organization custom domain registration. Organization scope is resolved from the authenticated provider token.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Custom-domain ID returned by the Ona API.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"domain_name": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Bare hostname registered as the custom domain, for example `ona.example.com`. URLs, paths, wildcard names, and trailing dots are rejected.",
			},
			"cloud_provider": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Cloud provider that owns the relay account for this custom domain. Supported values are `aws` and `gcp`.",
			},
			"cloud_account_id": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "AWS account ID or GCP project ID used for relay validation.",
			},
			"created_at": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp when the custom-domain registration was created.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"updated_at": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp when the custom-domain registration was last updated.",
			},
		},
	}
}

func (r *CustomDomainResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *CustomDomainResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	resp.Diagnostics.Append(validateCustomDomainConfig(ctx, req.Config)...)
}

func (r *CustomDomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data CustomDomainModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.requireClient(&resp.Diagnostics, "creating") {
		return
	}

	organizationID, err := r.authenticatedOrganizationID(ctx)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Ona Organization", "getting the authenticated organization for ona_custom_domain", err)
		return
	}

	provider, diags := customDomainProviderFromTerraform(data.CloudProvider, path.Root("cloud_provider"))
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudAccountID := data.CloudAccountID.ValueString()
	result, err := r.client.OrganizationService().CreateCustomDomain(ctx, connect.NewRequest(&v1.CreateCustomDomainRequest{
		OrganizationId: organizationID,
		DomainName:     data.DomainName.ValueString(),
		Provider:       provider,
		CloudAccountId: &cloudAccountID,
	}))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Create Ona Custom Domain", "creating the Ona custom domain", err)
		return
	}

	customDomain := result.Msg.GetCustomDomain()
	if customDomain == nil || customDomain.GetId() == "" {
		resp.Diagnostics.AddError("Unable to Create Ona Custom Domain", "The Ona API returned an empty custom-domain object after creation.")
		return
	}

	data.ID = types.StringValue(customDomain.GetId())
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	resp.Diagnostics.Append(setPrivateOrganizationID(ctx, resp.Private, organizationID)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(populateCustomDomainModel(&data, customDomain, organizationID)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CustomDomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data CustomDomainModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.requireClient(&resp.Diagnostics, "reading") {
		return
	}

	organizationID, err := r.authenticatedOrganizationID(ctx)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Ona Organization", "getting the authenticated organization for ona_custom_domain", err)
		return
	}
	resp.Diagnostics.Append(verifyPrivateOrganizationID(ctx, req.Private, organizationID)...)
	if resp.Diagnostics.HasError() {
		return
	}

	customDomain, err := r.getCustomDomain(ctx, organizationID)
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Custom Domain", "reading the Ona custom domain", err)
		return
	}

	data = CustomDomainModel{}
	resp.Diagnostics.Append(populateCustomDomainModel(&data, customDomain, organizationID)...)
	resp.Diagnostics.Append(setPrivateOrganizationID(ctx, resp.Private, organizationID)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CustomDomainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data CustomDomainModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.requireClient(&resp.Diagnostics, "updating") {
		return
	}

	organizationID, err := r.authenticatedOrganizationID(ctx)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Ona Organization", "getting the authenticated organization for ona_custom_domain", err)
		return
	}
	resp.Diagnostics.Append(verifyPrivateOrganizationID(ctx, req.Private, organizationID)...)
	if resp.Diagnostics.HasError() {
		return
	}

	provider, diags := customDomainProviderFromTerraform(data.CloudProvider, path.Root("cloud_provider"))
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	cloudAccountID := data.CloudAccountID.ValueString()
	result, err := r.client.OrganizationService().UpdateCustomDomain(ctx, connect.NewRequest(&v1.UpdateCustomDomainRequest{
		OrganizationId: organizationID,
		DomainName:     data.DomainName.ValueString(),
		CloudAccountId: &cloudAccountID,
		Provider:       &provider,
	}))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Update Ona Custom Domain", "updating the Ona custom domain", err)
		return
	}

	resp.Diagnostics.Append(populateCustomDomainModel(&data, result.Msg.GetCustomDomain(), organizationID)...)
	resp.Diagnostics.Append(setPrivateOrganizationID(ctx, resp.Private, organizationID)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CustomDomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data CustomDomainModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !r.requireClient(&resp.Diagnostics, "deleting") {
		return
	}

	organizationID, err := r.authenticatedOrganizationID(ctx)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Ona Organization", "getting the authenticated organization for ona_custom_domain", err)
		return
	}
	resp.Diagnostics.Append(verifyPrivateOrganizationID(ctx, req.Private, organizationID)...)
	if resp.Diagnostics.HasError() {
		return
	}

	_, err = r.client.OrganizationService().DeleteCustomDomain(ctx, connect.NewRequest(&v1.DeleteCustomDomainRequest{
		OrganizationId: organizationID,
	}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Delete Ona Custom Domain", "deleting the Ona custom domain", err)
		return
	}

	resp.Diagnostics.Append(setPrivateOrganizationID(ctx, resp.Private, "")...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.State.RemoveResource(ctx)
}

func (r *CustomDomainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	if !r.requireClient(&resp.Diagnostics, "importing") {
		return
	}

	organizationID, err := r.authenticatedOrganizationID(ctx)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Resolve Ona Organization", "getting the authenticated organization for ona_custom_domain", err)
		return
	}

	resp.Diagnostics.Append(validateCustomDomainImportID(req.ID, organizationID)...)
	if resp.Diagnostics.HasError() {
		return
	}

	customDomain, err := r.getCustomDomain(ctx, organizationID)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Import Ona Custom Domain", "reading the Ona custom domain for import", err)
		return
	}

	var data CustomDomainModel
	resp.Diagnostics.Append(populateCustomDomainModel(&data, customDomain, organizationID)...)
	resp.Diagnostics.Append(setPrivateOrganizationID(ctx, resp.Private, organizationID)...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *CustomDomainResource) requireClient(diags *diag.Diagnostics, action string) bool {
	if r.client != nil {
		return true
	}
	diags.AddError(
		"Ona API Client Is Not Configured",
		fmt.Sprintf("Set the provider token argument or ONA_TOKEN before %s ona_custom_domain resources.", action),
	)
	return false
}

func (r *CustomDomainResource) authenticatedOrganizationID(ctx context.Context) (string, error) {
	result, err := r.client.IdentityService().GetAuthenticatedIdentity(ctx, connect.NewRequest(&v1.GetAuthenticatedIdentityRequest{}))
	if err != nil {
		return "", fmt.Errorf("get authenticated identity: %w", err)
	}
	organizationID := result.Msg.GetOrganizationId()
	if organizationID == "" {
		return "", fmt.Errorf("authenticated identity did not include an organization ID")
	}
	return organizationID, nil
}

func (r *CustomDomainResource) getCustomDomain(ctx context.Context, organizationID string) (*v1.CustomDomain, error) {
	result, err := r.client.OrganizationService().GetCustomDomain(ctx, connect.NewRequest(&v1.GetCustomDomainRequest{
		OrganizationId: organizationID,
	}))
	if err != nil {
		return nil, fmt.Errorf("get custom domain: %w", err)
	}
	return result.Msg.GetCustomDomain(), nil
}

func populateCustomDomainModel(data *CustomDomainModel, customDomain *v1.CustomDomain, authenticatedOrganizationID string) diag.Diagnostics {
	var diags diag.Diagnostics
	if customDomain == nil {
		diags.AddError("Missing Custom Domain", "The Ona API returned an empty custom-domain object.")
		return diags
	}
	if organizationID := customDomain.GetOrganizationId(); organizationID != "" && organizationID != authenticatedOrganizationID {
		diags.AddError(
			"Custom Domain Organization Mismatch",
			fmt.Sprintf("The Ona API returned a custom domain for organization %q, but the authenticated provider token is scoped to organization %q.", organizationID, authenticatedOrganizationID),
		)
		return diags
	}

	cloudProvider, providerDiags := customDomainProviderToTerraform(customDomain.GetProvider(), path.Root("cloud_provider"))
	diags.Append(providerDiags...)
	if diags.HasError() {
		return diags
	}

	data.ID = types.StringValue(customDomain.GetId())
	data.DomainName = types.StringValue(customDomain.GetDomainName())
	data.CloudProvider = cloudProvider
	data.CloudAccountID = types.StringValue(customDomain.GetCloudAccountId())
	data.CreatedAt = timestampString(customDomain.GetCreatedAt())
	data.UpdatedAt = timestampString(customDomain.GetUpdatedAt())
	return diags
}

func customDomainProviderFromTerraform(value types.String, attrPath path.Path) (v1.CustomDomainProvider, diag.Diagnostics) {
	var diags diag.Diagnostics
	if value.IsNull() || value.IsUnknown() {
		return v1.CustomDomainProvider_CUSTOM_DOMAIN_PROVIDER_UNSPECIFIED, diags
	}
	switch value.ValueString() {
	case customDomainProviderAWS:
		return v1.CustomDomainProvider_CUSTOM_DOMAIN_PROVIDER_AWS, diags
	case customDomainProviderGCP:
		return v1.CustomDomainProvider_CUSTOM_DOMAIN_PROVIDER_GCP, diags
	default:
		diags.AddAttributeError(attrPath, "Invalid Cloud Provider", "cloud_provider must be either \"aws\" or \"gcp\".")
		return v1.CustomDomainProvider_CUSTOM_DOMAIN_PROVIDER_UNSPECIFIED, diags
	}
}

func customDomainProviderToTerraform(provider v1.CustomDomainProvider, attrPath path.Path) (types.String, diag.Diagnostics) {
	var diags diag.Diagnostics
	switch provider {
	case v1.CustomDomainProvider_CUSTOM_DOMAIN_PROVIDER_AWS:
		return types.StringValue(customDomainProviderAWS), diags
	case v1.CustomDomainProvider_CUSTOM_DOMAIN_PROVIDER_GCP:
		return types.StringValue(customDomainProviderGCP), diags
	default:
		diags.AddAttributeError(attrPath, "Unsupported Cloud Provider", fmt.Sprintf("The Ona API returned unsupported custom-domain provider %q.", provider.String()))
		return types.StringNull(), diags
	}
}

func validateCustomDomainConfig(ctx context.Context, cfg tfsdk.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	var domainName types.String
	diags.Append(cfg.GetAttribute(ctx, path.Root("domain_name"), &domainName)...)
	if !diags.HasError() {
		validateCustomDomainName(path.Root("domain_name"), domainName, &diags)
	}

	var cloudProvider types.String
	var cloudAccountID types.String
	diags.Append(cfg.GetAttribute(ctx, path.Root("cloud_provider"), &cloudProvider)...)
	diags.Append(cfg.GetAttribute(ctx, path.Root("cloud_account_id"), &cloudAccountID)...)
	if diags.HasError() {
		return diags
	}

	validateCustomDomainCloudProvider(path.Root("cloud_provider"), cloudProvider, &diags)
	validateCustomDomainCloudAccountID(path.Root("cloud_account_id"), cloudProvider, cloudAccountID, &diags)
	return diags
}

func validateCustomDomainName(attrPath path.Path, value types.String, diags *diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return
	}
	domainName := value.ValueString()
	if len(domainName) < 4 || len(domainName) > 253 {
		diags.AddAttributeError(attrPath, "Invalid Domain Name", "domain_name must be between 4 and 253 characters.")
		return
	}
	if strings.TrimSpace(domainName) != domainName {
		diags.AddAttributeError(attrPath, "Invalid Domain Name", "domain_name must not contain leading or trailing whitespace.")
		return
	}
	if parsed, err := url.Parse(domainName); err == nil && parsed.Scheme != "" {
		diags.AddAttributeError(attrPath, "Invalid Domain Name", "domain_name must be a bare hostname, not a URL.")
		return
	}
	if strings.ContainsAny(domainName, "/?#") {
		diags.AddAttributeError(attrPath, "Invalid Domain Name", "domain_name must be a bare hostname without a path, query string, or fragment.")
		return
	}
	if strings.Contains(domainName, "*") {
		diags.AddAttributeError(attrPath, "Invalid Domain Name", "domain_name must not be a wildcard hostname.")
		return
	}
	if strings.HasSuffix(domainName, ".") || !customDomainNamePattern.MatchString(domainName) {
		diags.AddAttributeError(attrPath, "Invalid Domain Name", "domain_name must be a bare hostname with labels that start and end with a letter or digit.")
	}
}

func validateCustomDomainCloudProvider(attrPath path.Path, value types.String, diags *diag.Diagnostics) {
	if value.IsNull() || value.IsUnknown() {
		return
	}
	if value.ValueString() != customDomainProviderAWS && value.ValueString() != customDomainProviderGCP {
		diags.AddAttributeError(attrPath, "Invalid Cloud Provider", "cloud_provider must be either \"aws\" or \"gcp\".")
	}
}

func validateCustomDomainCloudAccountID(attrPath path.Path, cloudProvider types.String, cloudAccountID types.String, diags *diag.Diagnostics) {
	if cloudAccountID.IsUnknown() || cloudProvider.IsUnknown() || cloudProvider.IsNull() {
		return
	}
	if cloudAccountID.IsNull() || cloudAccountID.ValueString() == "" {
		diags.AddAttributeError(attrPath, "Invalid Cloud Account ID", "cloud_account_id must not be empty.")
		return
	}
	accountID := cloudAccountID.ValueString()
	switch cloudProvider.ValueString() {
	case customDomainProviderAWS:
		if !awsCloudAccountIDPattern.MatchString(accountID) {
			diags.AddAttributeError(attrPath, "Invalid AWS Account ID", "cloud_account_id must be exactly 12 digits when cloud_provider is \"aws\".")
		}
	case customDomainProviderGCP:
		if !gcpCloudAccountIDPattern.MatchString(accountID) {
			diags.AddAttributeError(attrPath, "Invalid GCP Project ID", "cloud_account_id must be 6-30 lowercase letters, numbers, or hyphens, start with a letter, and not end with a hyphen when cloud_provider is \"gcp\".")
		}
	}
}

func validateCustomDomainImportID(importID string, authenticatedOrganizationID string) diag.Diagnostics {
	var diags diag.Diagnostics
	if importID == "" || strings.TrimSpace(importID) != importID || strings.Contains(importID, "/") {
		diags.AddError("Invalid Import ID", "Use \"current\" to import the custom domain for the authenticated organization.")
		return diags
	}
	if importID == customDomainImportCurrent || importID == authenticatedOrganizationID {
		return diags
	}
	diags.AddError(
		"Invalid Import ID",
		fmt.Sprintf("Use \"current\" or the authenticated organization ID %q to import ona_custom_domain. The provider token cannot import custom domains for other organizations.", authenticatedOrganizationID),
	)
	return diags
}

func setPrivateOrganizationID(ctx context.Context, state privateState, organizationID string) diag.Diagnostics {
	var diags diag.Diagnostics
	if state == nil {
		diags.AddError("Unable to Store Private State", "Terraform did not provide private state storage for ona_custom_domain.")
		return diags
	}
	if organizationID == "" {
		diags.Append(state.SetKey(ctx, customDomainPrivateOrganizationIDKey, nil)...)
		return diags
	}
	data, err := json.Marshal(organizationID)
	if err != nil {
		diags.AddError("Unable to Store Private State", fmt.Sprintf("Could not encode authenticated organization ID: %s.", err))
		return diags
	}
	diags.Append(state.SetKey(ctx, customDomainPrivateOrganizationIDKey, data)...)
	return diags
}

func verifyPrivateOrganizationID(ctx context.Context, state privateState, authenticatedOrganizationID string) diag.Diagnostics {
	var diags diag.Diagnostics
	if state == nil {
		return diags
	}
	data, stateDiags := state.GetKey(ctx, customDomainPrivateOrganizationIDKey)
	diags.Append(stateDiags...)
	if diags.HasError() || len(data) == 0 {
		return diags
	}

	var storedOrganizationID string
	if err := json.Unmarshal(data, &storedOrganizationID); err != nil {
		diags.AddError("Unable to Read Private State", fmt.Sprintf("Could not decode stored organization scope for ona_custom_domain: %s.", err))
		return diags
	}
	if storedOrganizationID != "" && storedOrganizationID != authenticatedOrganizationID {
		diags.AddError(
			"Custom Domain Organization Scope Changed",
			fmt.Sprintf("This ona_custom_domain resource was created or imported with a token scoped to organization %q, but the provider token is now scoped to organization %q. Import or create a separate resource for the new organization instead of reusing this state.", storedOrganizationID, authenticatedOrganizationID),
		)
	}
	return diags
}
