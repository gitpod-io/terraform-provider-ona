// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package gitauthentication

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"
	v1 "github.com/gitpod-io/gitpod-sdk-go/v1"
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/listutil"
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

var _ resource.Resource = &Resource{}
var _ resource.ResourceWithConfigure = &Resource{}
var _ resource.ResourceWithIdentity = &Resource{}
var _ resource.ResourceWithImportState = &Resource{}

func NewResource() resource.Resource { return &Resource{} }

type Resource struct {
	client     *managementclient.ManagementPlane
	apiBaseURL string
	userAgent  string
}

type Model struct {
	ID                         types.String `tfsdk:"id"`
	ServiceAccountID           types.String `tfsdk:"service_account_id"`
	SCMIntegrationID           types.String `tfsdk:"scm_integration_id"`
	RunnerID                   types.String `tfsdk:"runner_id"`
	Host                       types.String `tfsdk:"host"`
	PersonalAccessToken        types.String `tfsdk:"personal_access_token"`
	PersonalAccessTokenVersion types.String `tfsdk:"personal_access_token_version"`
	ExpiresAt                  types.String `tfsdk:"expires_at"`
	Scopes                     types.Set    `tfsdk:"scopes"`
}

func (r *Resource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_git_authentication"
}

func (r *Resource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = resourceschema.Schema{
		MarkdownDescription: "PAT-backed Git authentication for an Ona service account through a PAT-enabled runner SCM integration.",
		Attributes: map[string]resourceschema.Attribute{
			"id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Git authentication ID.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"service_account_id": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Service account that uses this Git authentication. Changing this value replaces the authentication.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"scm_integration_id": resourceschema.StringAttribute{
				Required:            true,
				MarkdownDescription: "PAT-enabled SCM integration that determines the runner and host. Changing this value replaces the authentication.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"runner_id": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Runner derived from the SCM integration.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"host": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "SCM host derived from the SCM integration.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"personal_access_token": resourceschema.StringAttribute{
				Optional:            true,
				Sensitive:           true,
				WriteOnly:           true,
				MarkdownDescription: "SCM personal access token. This write-only value is sent to Ona but is not stored in Terraform plan or state. Required on create and when changing `personal_access_token_version`.",
			},
			"personal_access_token_version": resourceschema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "User-managed version marker for resubmitting `personal_access_token` during rotation.",
			},
			"expires_at": resourceschema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "RFC3339 timestamp when the PAT expires, when known.",
			},
			"scopes": resourceschema.SetAttribute{
				Computed:            true,
				ElementType:         types.StringType,
				MarkdownDescription: "Non-secret PAT scope metadata returned by Ona.",
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
		resp.Diagnostics.AddError("Unexpected Resource Configure Type", fmt.Sprintf("Expected *providerdata.Data, got: %T. Please report this issue to the provider developers.", req.ProviderData))
		return
	}
	r.client = data.Client
	r.apiBaseURL = data.APIBaseURL
	r.userAgent = data.UserAgent
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	pat := readPersonalAccessToken(ctx, req.Config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(validateCreateInput(data, pat)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "creating") {
		return
	}

	integration, err := r.getSCMIntegration(ctx, data.SCMIntegrationID.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona SCM Integration", "reading the SCM integration for Git authentication", err)
		return
	}
	resp.Diagnostics.Append(validateSCMIntegration(integration, data.SCMIntegrationID.ValueString())...)
	if resp.Diagnostics.HasError() {
		return
	}

	serviceAccountClient, err := r.serviceAccountClient(ctx, data.ServiceAccountID.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Authenticate as Ona Service Account", "creating a temporary access token for Git authentication", err)
		return
	}

	createReq := createHostAuthenticationTokenRequest(data, integration, pat)
	result, err := serviceAccountClient.RunnerConfigurationService().CreateHostAuthenticationToken(ctx, connect.NewRequest(createReq))
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Create Ona Git Authentication", "creating the service-account Git authentication", err)
		return
	}
	created := result.Msg.GetToken()
	if created == nil || created.GetId() == "" {
		resp.Diagnostics.AddError("Unable to Create Ona Git Authentication", "The Ona API returned an empty Git authentication.")
		return
	}

	data.ID = types.StringValue(created.GetId())
	data.RunnerID = types.StringValue(integration.GetRunnerId())
	data.Host = types.StringValue(integration.GetHost())
	data.PersonalAccessToken = types.StringNull()
	data.ExpiresAt = types.StringNull()
	data.Scopes = types.SetNull(types.StringType)
	resp.Diagnostics.Append(resp.Identity.Set(ctx, IdentityModel{ID: data.ID})...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(validateHostAuthenticationToken(created, integration, data.ServiceAccountID.ValueString(), data.SCMIntegrationID.ValueString())...)
	if resp.Diagnostics.HasError() {
		return
	}
	data = modelFromRemote(ctx, created, integration.GetId(), data.PersonalAccessTokenVersion, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, IdentityModel{ID: data.ID})...)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var prior Model
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "reading") {
		return
	}
	if prior.ID.ValueString() == "" {
		resp.Diagnostics.AddError("Unable to Read Ona Git Authentication", "Git authentication ID is empty.")
		return
	}

	serviceAccountID := prior.ServiceAccountID.ValueString()
	if serviceAccountID == "" {
		remote, err := getHostAuthenticationToken(ctx, r.client, prior.ID.ValueString())
		if err != nil {
			providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Git Authentication", "discovering the service account for imported Git authentication", err)
			return
		}
		if remote == nil {
			resp.State.RemoveResource(ctx)
			return
		}
		subject := remote.GetSubject()
		if subject == nil || subject.GetId() == "" || subject.GetPrincipal() != v1.Principal_PRINCIPAL_SERVICE_ACCOUNT {
			resp.Diagnostics.AddError("Unsupported Ona Git Authentication Subject", fmt.Sprintf("Git authentication %q is not owned by a service account.", remote.GetId()))
			return
		}
		serviceAccountID = subject.GetId()
	}

	serviceAccountClient, err := r.serviceAccountClient(ctx, serviceAccountID)
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Authenticate as Ona Service Account", "creating a temporary access token for reading Git authentication", err)
		return
	}

	remote, integration, err := r.getValidatedRemote(ctx, serviceAccountClient, prior.ID.ValueString(), prior.SCMIntegrationID.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Git Authentication", "reading the service-account Git authentication", err)
		return
	}
	if remote == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(validateHostAuthenticationToken(remote, integration, serviceAccountID, prior.SCMIntegrationID.ValueString())...)
	if resp.Diagnostics.HasError() {
		return
	}
	data := modelFromRemote(ctx, remote, integration.GetId(), prior.PersonalAccessTokenVersion, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, IdentityModel{ID: data.ID})...)
	if resp.Diagnostics.HasError() {
		return
	}
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
	pat := readPersonalAccessToken(ctx, req.Config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.requireClient(&resp.Diagnostics, "updating") {
		return
	}

	serviceAccountClient, err := r.serviceAccountClient(ctx, prior.ServiceAccountID.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Authenticate as Ona Service Account", "creating a temporary access token for updating Git authentication", err)
		return
	}

	remote, integration, err := r.getValidatedRemote(ctx, serviceAccountClient, prior.ID.ValueString(), prior.SCMIntegrationID.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Ona Git Authentication", "reading the service-account Git authentication before rotation", err)
		return
	}
	if remote == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(validateHostAuthenticationToken(remote, integration, prior.ServiceAccountID.ValueString(), prior.SCMIntegrationID.ValueString())...)
	if resp.Diagnostics.HasError() {
		return
	}

	rotated := secretVersionChanged(data.PersonalAccessTokenVersion, prior.PersonalAccessTokenVersion)
	resp.Diagnostics.Append(validateUpdateInput(rotated, pat)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if rotated {
		if _, err := serviceAccountClient.RunnerConfigurationService().UpdateHostAuthenticationToken(ctx, connect.NewRequest(&v1.UpdateHostAuthenticationTokenRequest{
			Id:    prior.ID.ValueString(),
			Token: stringPointer(pat.ValueString()),
		})); err != nil {
			providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Update Ona Git Authentication", "rotating the service-account Git authentication", err)
			return
		}
		resp.Diagnostics.Append(resp.Identity.Set(ctx, IdentityModel{ID: prior.ID})...)
		if resp.Diagnostics.HasError() {
			return
		}
		resp.Diagnostics.Append(resp.State.Set(ctx, &prior)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	if rotated {
		remote, err = getHostAuthenticationTokenAfterWrite(ctx, prior.ID.ValueString(), func(ctx context.Context, id string) (*v1.HostAuthenticationToken, error) {
			return getHostAuthenticationToken(ctx, serviceAccountClient, id)
		}, waitForRetry)
	} else {
		remote, err = getHostAuthenticationToken(ctx, serviceAccountClient, prior.ID.ValueString())
	}
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Read Updated Ona Git Authentication", "reading the updated service-account Git authentication", err)
		return
	}
	if remote == nil {
		if rotated {
			resp.Diagnostics.AddError(
				"Unable to Read Updated Ona Git Authentication",
				fmt.Sprintf("Ona accepted the personal access token rotation, but Git authentication %q was not visible after %d read attempts. Terraform retained the prior state so it does not lose track of the remote authentication. Retry the apply after the Ona read replica has synchronized.", prior.ID.ValueString(), postWriteReadMaxAttempts),
			)
			return
		}
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(validateHostAuthenticationToken(remote, integration, data.ServiceAccountID.ValueString(), data.SCMIntegrationID.ValueString())...)
	if resp.Diagnostics.HasError() {
		return
	}
	data = modelFromRemote(ctx, remote, integration.GetId(), data.PersonalAccessTokenVersion, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, IdentityModel{ID: data.ID})...)
	if resp.Diagnostics.HasError() {
		return
	}
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
	serviceAccountClient, err := r.serviceAccountClient(ctx, data.ServiceAccountID.ValueString())
	if err != nil {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Authenticate as Ona Service Account", "creating a temporary access token for deleting Git authentication", err)
		return
	}
	_, err = serviceAccountClient.RunnerConfigurationService().DeleteHostAuthenticationToken(ctx, connect.NewRequest(&v1.DeleteHostAuthenticationTokenRequest{Id: data.ID.ValueString()}))
	if err != nil && connect.CodeOf(err) != connect.CodeNotFound {
		providerdiag.AddAPIError(&resp.Diagnostics, "Unable to Delete Ona Git Authentication", "deleting the service-account Git authentication", err)
		return
	}
	resp.State.RemoveResource(ctx)
}

func (r *Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughWithIdentity(ctx, path.Root("id"), path.Root("id"), req, resp)
}

func (r *Resource) requireClient(diags *diag.Diagnostics, operation string) bool {
	if r.client != nil {
		return true
	}
	diags.AddError("Ona API Client Is Not Configured", fmt.Sprintf("Set the provider token argument or ONA_TOKEN before %s ona_git_authentication resources.", operation))
	return false
}

func getHostAuthenticationToken(ctx context.Context, client *managementclient.ManagementPlane, id string) (*v1.HostAuthenticationToken, error) {
	result, err := client.RunnerConfigurationService().GetHostAuthenticationToken(ctx, connect.NewRequest(&v1.GetHostAuthenticationTokenRequest{Id: id}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get host authentication token: %w", err)
	}
	return result.Msg.GetToken(), nil
}

const (
	postWriteReadMaxAttempts  = 4
	postWriteReadInitialDelay = 100 * time.Millisecond
)

type hostAuthenticationTokenGetter func(context.Context, string) (*v1.HostAuthenticationToken, error)
type retryWaiter func(context.Context, time.Duration) error

func getHostAuthenticationTokenAfterWrite(ctx context.Context, id string, get hostAuthenticationTokenGetter, wait retryWaiter) (*v1.HostAuthenticationToken, error) {
	delay := postWriteReadInitialDelay
	for attempt := range postWriteReadMaxAttempts {
		token, err := get(ctx, id)
		if err != nil || token != nil {
			return token, err
		}
		if attempt == postWriteReadMaxAttempts-1 {
			return nil, nil
		}
		if err := wait(ctx, delay); err != nil {
			return nil, fmt.Errorf("wait before retrying host authentication token read: %w", err)
		}
		delay *= 2
	}
	return nil, nil
}

func waitForRetry(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (r *Resource) getSCMIntegration(ctx context.Context, id string) (*v1.SCMIntegration, error) {
	result, err := r.client.RunnerConfigurationService().GetSCMIntegration(ctx, connect.NewRequest(&v1.GetSCMIntegrationRequest{Id: id}))
	if err != nil {
		if connect.CodeOf(err) == connect.CodeNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get SCM integration: %w", err)
	}
	return result.Msg.GetIntegration(), nil
}

func (r *Resource) serviceAccountClient(ctx context.Context, serviceAccountID string) (*managementclient.ManagementPlane, error) {
	if r.apiBaseURL == "" {
		return nil, fmt.Errorf("ona API base URL is not configured")
	}
	result, err := r.client.ServiceAccountService().CreateServiceAccountAccessToken(ctx, connect.NewRequest(&v1.CreateServiceAccountAccessTokenRequest{
		ServiceAccountId: serviceAccountID,
	}))
	if err != nil {
		return nil, fmt.Errorf("create service account access token: %w", err)
	}
	if result == nil || result.Msg == nil || result.Msg.GetToken() == "" {
		return nil, fmt.Errorf("create service account access token: Ona API returned an empty token")
	}
	client, err := managementclient.New(r.apiBaseURL,
		managementclient.WithAccessToken(result.Msg.GetToken()),
		managementclient.WithUserAgent(r.userAgent),
	)
	if err != nil {
		return nil, fmt.Errorf("configure service account client: %w", err)
	}
	return client, nil
}

func (r *Resource) getValidatedRemote(ctx context.Context, client *managementclient.ManagementPlane, id string, integrationID string) (*v1.HostAuthenticationToken, *v1.SCMIntegration, error) {
	remote, err := getHostAuthenticationToken(ctx, client, id)
	if err != nil || remote == nil {
		return remote, nil, err
	}
	if integrationID != "" {
		integration, err := r.getSCMIntegration(ctx, integrationID)
		return remote, integration, err
	}
	integration, err := r.resolveSCMIntegration(ctx, remote.GetRunnerId(), remote.GetHost())
	if err != nil {
		return nil, nil, err
	}
	if integration == nil {
		return nil, nil, fmt.Errorf("no PAT-enabled SCM integration found for runner %q and host %q", remote.GetRunnerId(), remote.GetHost())
	}
	return remote, integration, nil
}

func (r *Resource) resolveSCMIntegration(ctx context.Context, runnerID string, host string) (*v1.SCMIntegration, error) {
	if runnerID == "" || host == "" {
		return nil, fmt.Errorf("git authentication must include a non-empty runner ID and host")
	}

	candidateIDs := make(map[string]struct{})
	seenTokens := make(map[string]struct{})
	var paginationToken string
	for {
		result, err := r.client.RunnerConfigurationService().ListSCMIntegrations(ctx, connect.NewRequest(&v1.ListSCMIntegrationsRequest{
			Pagination: &v1.PaginationRequest{PageSize: listutil.DefaultPageSize, Token: paginationToken},
			Filter:     &v1.ListSCMIntegrationsRequest_Filter{RunnerIds: []string{runnerID}},
		}))
		if err != nil {
			return nil, fmt.Errorf("list SCM integrations for runner %q: %w", runnerID, err)
		}
		for _, integration := range result.Msg.GetIntegrations() {
			if integration.GetId() != "" && integration.GetRunnerId() == runnerID && integration.GetHost() == host && integration.GetPat() {
				candidateIDs[integration.GetId()] = struct{}{}
			}
		}

		nextToken := result.Msg.GetPagination().GetNextToken()
		if err := listutil.NextPageToken(seenTokens, nextToken); err != nil {
			return nil, err
		}
		if nextToken == "" {
			break
		}
		paginationToken = nextToken
	}

	var match *v1.SCMIntegration
	for candidateID := range candidateIDs {
		integration, err := r.getSCMIntegration(ctx, candidateID)
		if err != nil {
			return nil, fmt.Errorf("verify SCM integration %q: %w", candidateID, err)
		}
		if integration == nil || integration.GetRunnerId() != runnerID || integration.GetHost() != host || !integration.GetPat() {
			continue
		}
		if match != nil {
			return nil, fmt.Errorf("multiple PAT-enabled SCM integrations found for runner %q and host %q", runnerID, host)
		}
		match = integration
	}
	return match, nil
}

func readPersonalAccessToken(ctx context.Context, cfg tfsdk.Config, diags *diag.Diagnostics) types.String {
	var value types.String
	diags.Append(cfg.GetAttribute(ctx, path.Root("personal_access_token"), &value)...)
	return value
}

func knownString(value types.String) bool {
	return !value.IsNull() && !value.IsUnknown() && value.ValueString() != ""
}

func validateCreateInput(data Model, pat types.String) diag.Diagnostics {
	var diags diag.Diagnostics
	if !knownString(data.ServiceAccountID) {
		diags.AddAttributeError(path.Root("service_account_id"), "Missing Service Account ID", "Set service_account_id to a known, non-empty Ona service account ID.")
	}
	if !knownString(pat) {
		diags.AddAttributeError(path.Root("personal_access_token"), "Missing Personal Access Token", "Set personal_access_token when creating ona_git_authentication resources.")
	}
	return diags
}

func validateUpdateInput(rotated bool, pat types.String) diag.Diagnostics {
	var diags diag.Diagnostics
	if rotated && !knownString(pat) {
		diags.AddAttributeError(path.Root("personal_access_token"), "Missing Personal Access Token", "Set personal_access_token when changing personal_access_token_version.")
	}
	return diags
}

func createHostAuthenticationTokenRequest(data Model, integration *v1.SCMIntegration, pat types.String) *v1.CreateHostAuthenticationTokenRequest {
	return &v1.CreateHostAuthenticationTokenRequest{
		RunnerId: integration.GetRunnerId(),
		Host:     integration.GetHost(),
		Token:    pat.ValueString(),
		Source:   v1.HostAuthenticationTokenSource_HOST_AUTHENTICATION_TOKEN_SOURCE_PAT,
		Subject: &v1.Subject{
			Id:        data.ServiceAccountID.ValueString(),
			Principal: v1.Principal_PRINCIPAL_SERVICE_ACCOUNT,
		},
	}
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

func validateSCMIntegration(integration *v1.SCMIntegration, expectedID string) diag.Diagnostics {
	var diags diag.Diagnostics
	if integration == nil {
		diags.AddError("Unable to Use Ona SCM Integration", fmt.Sprintf("SCM integration %q was not found.", expectedID))
		return diags
	}
	if integration.GetId() == "" || integration.GetRunnerId() == "" || integration.GetHost() == "" {
		diags.AddError("Invalid Ona SCM Integration", "The SCM integration must include a non-empty ID, runner ID, and host.")
	}
	if expectedID != "" && integration.GetId() != expectedID {
		diags.AddError("Unexpected Ona SCM Integration", fmt.Sprintf("Ona returned SCM integration %q while Terraform requested %q.", integration.GetId(), expectedID))
	}
	if !integration.GetPat() {
		diags.AddError("SCM Integration Does Not Support PAT Authentication", fmt.Sprintf("SCM integration %q does not have PAT authentication enabled.", expectedID))
	}
	return diags
}

func validateHostAuthenticationToken(token *v1.HostAuthenticationToken, integration *v1.SCMIntegration, expectedServiceAccountID string, expectedIntegrationID string) diag.Diagnostics {
	var diags diag.Diagnostics
	if token == nil || token.GetId() == "" {
		diags.AddError("Invalid Ona Git Authentication", "Ona returned an empty Git authentication.")
		return diags
	}
	diags.Append(validateSCMIntegration(integration, expectedIntegrationID)...)
	if token.GetSource() != v1.HostAuthenticationTokenSource_HOST_AUTHENTICATION_TOKEN_SOURCE_PAT {
		diags.AddError("Unsupported Ona Git Authentication Source", fmt.Sprintf("Git authentication %q is not PAT-backed.", token.GetId()))
	}
	subject := token.GetSubject()
	if subject == nil || subject.GetId() == "" || subject.GetPrincipal() != v1.Principal_PRINCIPAL_SERVICE_ACCOUNT {
		diags.AddError("Unsupported Ona Git Authentication Subject", fmt.Sprintf("Git authentication %q is not owned by a service account.", token.GetId()))
	} else if expectedServiceAccountID != "" && subject.GetId() != expectedServiceAccountID {
		diags.AddError("Unexpected Ona Git Authentication Subject", fmt.Sprintf("Git authentication %q belongs to service account %q, not %q.", token.GetId(), subject.GetId(), expectedServiceAccountID))
	}
	if integration != nil && (token.GetRunnerId() != integration.GetRunnerId() || token.GetHost() != integration.GetHost()) {
		diags.AddError("Inconsistent Ona Git Authentication Target", fmt.Sprintf("Git authentication %q does not match SCM integration %q's runner and host.", token.GetId(), integration.GetId()))
	}
	return diags
}

func modelFromRemote(ctx context.Context, token *v1.HostAuthenticationToken, scmIntegrationID string, version types.String, diags *diag.Diagnostics) Model {
	scopes, scopeDiags := types.SetValueFrom(ctx, types.StringType, token.GetScopes())
	diags.Append(scopeDiags...)
	return Model{
		ID:                         types.StringValue(token.GetId()),
		ServiceAccountID:           types.StringValue(token.GetSubject().GetId()),
		SCMIntegrationID:           types.StringValue(scmIntegrationID),
		RunnerID:                   types.StringValue(token.GetRunnerId()),
		Host:                       types.StringValue(token.GetHost()),
		PersonalAccessToken:        types.StringNull(),
		PersonalAccessTokenVersion: version,
		ExpiresAt:                  timestampValue(token.GetExpiresAt()),
		Scopes:                     scopes,
	}
}

func timestampValue(value interface {
	AsTime() time.Time
	IsValid() bool
}) types.String {
	if value == nil || !value.IsValid() {
		return types.StringNull()
	}
	return types.StringValue(value.AsTime().UTC().Format(time.RFC3339Nano))
}

func stringPointer(value string) *string { return &value }
