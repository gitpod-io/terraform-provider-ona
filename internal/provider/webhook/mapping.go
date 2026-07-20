// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package webhook

import (
	"context"
	"fmt"
	"sort"

	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func createWebhookRequest(ctx context.Context, data Model) (*v1.CreateWebhookRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	validateModel(ctx, data, true, &diags)
	if diags.HasError() {
		return nil, diags
	}

	webhookType, ok := webhookTypeFromString(data.Type.ValueString())
	if !ok {
		diags.AddAttributeError(path.Root("type"), "Invalid Webhook Type", "Supported values are \"repository\" and \"organization\".")
		return nil, diags
	}
	provider, ok := webhookProviderFromString(data.Provider.ValueString())
	if !ok {
		diags.AddAttributeError(path.Root("scm_provider"), "Invalid Webhook Provider", "Supported values are \"github\", \"gitlab\", and \"bitbucket\".")
		return nil, diags
	}

	req := &v1.CreateWebhookRequest{
		Name:        data.Name.ValueString(),
		Description: optionalString(data.Description),
		Type:        webhookType,
		Provider:    provider,
	}
	setWebhookScopes(ctx, data, &req.Scopes, &req.OrganizationScope, &diags)
	return req, diags
}

func updateWebhookRequest(ctx context.Context, data Model) (*v1.UpdateWebhookRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	validateModel(ctx, data, true, &diags)
	if diags.HasError() {
		return nil, diags
	}

	name := data.Name.ValueString()
	description := optionalString(data.Description)
	req := &v1.UpdateWebhookRequest{
		WebhookId:   data.ID.ValueString(),
		Name:        &name,
		Description: &description,
	}
	setWebhookScopes(ctx, data, &req.Scopes, &req.OrganizationScope, &diags)
	return req, diags
}

func setWebhookScopes(ctx context.Context, data Model, repositories *[]*v1.WebhookRepositoryScope, organization **v1.WebhookOrganizationScope, diags *diag.Diagnostics) {
	switch data.Type.ValueString() {
	case webhookTypeRepository:
		var scopes []RepositoryScopeModel
		diags.Append(data.RepositoryScopes.ElementsAs(ctx, &scopes, false)...)
		if diags.HasError() {
			return
		}
		sort.Slice(scopes, func(i, j int) bool {
			left := scopes[i].Host.ValueString() + "\x00" + scopes[i].Owner.ValueString() + "\x00" + scopes[i].Name.ValueString()
			right := scopes[j].Host.ValueString() + "\x00" + scopes[j].Owner.ValueString() + "\x00" + scopes[j].Name.ValueString()
			return left < right
		})
		*repositories = make([]*v1.WebhookRepositoryScope, 0, len(scopes))
		for _, scope := range scopes {
			*repositories = append(*repositories, &v1.WebhookRepositoryScope{
				Host:  scope.Host.ValueString(),
				Owner: scope.Owner.ValueString(),
				Name:  scope.Name.ValueString(),
			})
		}
	case webhookTypeOrganization:
		var scope OrganizationScopeModel
		diags.Append(data.OrganizationScope.As(ctx, &scope, basetypes.ObjectAsOptions{})...)
		if diags.HasError() {
			return
		}
		*organization = &v1.WebhookOrganizationScope{
			Host: scope.Host.ValueString(),
			Name: scope.Name.ValueString(),
		}
	}
}

func populateModel(ctx context.Context, data *Model, webhook *v1.Webhook, diags *diag.Diagnostics) {
	if webhook == nil {
		diags.AddError("Unable to Read Ona Webhook", "The Ona API returned an empty webhook.")
		return
	}

	metadata := webhook.GetMetadata()
	spec := webhook.GetSpec()
	webhookType, ok := webhookTypeToString(spec.GetType())
	if !ok {
		diags.AddError("Unable to Read Ona Webhook", fmt.Sprintf("The Ona API returned unsupported webhook type %q.", spec.GetType().String()))
		return
	}
	provider, ok := webhookProviderToString(spec.GetProvider())
	if !ok {
		diags.AddError("Unable to Read Ona Webhook", fmt.Sprintf("The Ona API returned unsupported webhook provider %q.", spec.GetProvider().String()))
		return
	}

	data.ID = stringValue(webhook.GetId())
	data.Name = stringValue(metadata.GetName())
	data.Description = optionalStringValue(metadata.GetDescription())
	data.Type = types.StringValue(webhookType)
	data.Provider = types.StringValue(provider)
	data.URL = stringValue(webhook.GetUrl())
	data.Creator = creatorObject(metadata.GetCreator(), diags)
	data.CreatedAt = timestampValue(metadata.GetCreatedAt())
	data.SecretVersion = types.StringNull()

	data.RepositoryScopes = types.SetNull(types.ObjectType{AttrTypes: repositoryScopeAttributeTypes})
	data.OrganizationScope = types.ObjectNull(organizationScopeAttributeTypes)
	if webhookType == webhookTypeRepository {
		scopes := make([]RepositoryScopeModel, 0, len(spec.GetScopes()))
		for _, scope := range spec.GetScopes() {
			scopes = append(scopes, RepositoryScopeModel{
				Host:  stringValue(scope.GetHost()),
				Owner: stringValue(scope.GetOwner()),
				Name:  stringValue(scope.GetName()),
			})
		}
		value, setDiags := types.SetValueFrom(ctx, types.ObjectType{AttrTypes: repositoryScopeAttributeTypes}, scopes)
		diags.Append(setDiags...)
		data.RepositoryScopes = value
	} else if scope := spec.GetOrganizationScope(); scope != nil {
		value, objectDiags := types.ObjectValueFrom(ctx, organizationScopeAttributeTypes, OrganizationScopeModel{
			Host: stringValue(scope.GetHost()),
			Name: stringValue(scope.GetName()),
		})
		diags.Append(objectDiags...)
		data.OrganizationScope = value
	}
}

func preservePlannedInputs(data *Model, planned Model) {
	if !planned.Description.IsUnknown() {
		data.Description = planned.Description
	}
	data.SecretVersion = planned.SecretVersion
}

func preserveTerraformOnlyState(data *Model, prior Model) {
	if data.Description.IsNull() && !prior.Description.IsUnknown() && (prior.Description.IsNull() || prior.Description.ValueString() == "") {
		data.Description = prior.Description
	}
	data.SecretVersion = prior.SecretVersion
}

func creatorObject(subject *v1.Subject, diags *diag.Diagnostics) types.Object {
	if subject == nil {
		return types.ObjectNull(creatorAttributeTypes)
	}
	principal := principalToString(subject.GetPrincipal())
	values := map[string]attr.Value{
		"id":        stringValue(subject.GetId()),
		"principal": optionalStringValue(principal),
	}
	result, objectDiags := types.ObjectValue(creatorAttributeTypes, values)
	diags.Append(objectDiags...)
	return result
}

func principalToString(principal v1.Principal) string {
	switch principal {
	case v1.Principal_PRINCIPAL_USER:
		return "user"
	case v1.Principal_PRINCIPAL_SERVICE_ACCOUNT:
		return "service_account"
	case v1.Principal_PRINCIPAL_ACCOUNT:
		return "account"
	case v1.Principal_PRINCIPAL_RUNNER:
		return "runner"
	default:
		return ""
	}
}

func webhookTypeFromString(value string) (v1.WebhookType, bool) {
	switch value {
	case webhookTypeRepository:
		return v1.WebhookType_WEBHOOK_TYPE_SCM_REPOSITORY, true
	case webhookTypeOrganization:
		return v1.WebhookType_WEBHOOK_TYPE_SCM_ORGANIZATION, true
	default:
		return v1.WebhookType_WEBHOOK_TYPE_UNSPECIFIED, false
	}
}

func webhookTypeToString(value v1.WebhookType) (string, bool) {
	switch value {
	case v1.WebhookType_WEBHOOK_TYPE_SCM_REPOSITORY:
		return webhookTypeRepository, true
	case v1.WebhookType_WEBHOOK_TYPE_SCM_ORGANIZATION:
		return webhookTypeOrganization, true
	default:
		return "", false
	}
}

func webhookProviderFromString(value string) (v1.WebhookProvider, bool) {
	switch value {
	case webhookProviderGitHub:
		return v1.WebhookProvider_WEBHOOK_PROVIDER_GITHUB, true
	case webhookProviderGitLab:
		return v1.WebhookProvider_WEBHOOK_PROVIDER_GITLAB, true
	case webhookProviderBitbucket:
		return v1.WebhookProvider_WEBHOOK_PROVIDER_BITBUCKET, true
	default:
		return v1.WebhookProvider_WEBHOOK_PROVIDER_UNSPECIFIED, false
	}
}

func webhookProviderToString(value v1.WebhookProvider) (string, bool) {
	switch value {
	case v1.WebhookProvider_WEBHOOK_PROVIDER_GITHUB:
		return webhookProviderGitHub, true
	case v1.WebhookProvider_WEBHOOK_PROVIDER_GITLAB:
		return webhookProviderGitLab, true
	case v1.WebhookProvider_WEBHOOK_PROVIDER_BITBUCKET:
		return webhookProviderBitbucket, true
	default:
		return "", false
	}
}

func optionalString(value types.String) string {
	if value.IsNull() || value.IsUnknown() {
		return ""
	}
	return value.ValueString()
}

func stringValue(value string) types.String {
	if value == "" {
		return types.StringNull()
	}
	return types.StringValue(value)
}

func optionalStringValue(value string) types.String {
	if value == "" {
		return types.StringNull()
	}
	return types.StringValue(value)
}

func timestampValue(value *timestamppb.Timestamp) types.String {
	if value == nil || !value.IsValid() {
		return types.StringNull()
	}
	return types.StringValue(value.AsTime().Format("2006-01-02T15:04:05Z07:00"))
}
