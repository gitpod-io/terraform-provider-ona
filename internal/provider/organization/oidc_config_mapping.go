// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package organization

import (
	"context"
	"fmt"
	"sort"
	"strings"

	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var oidcCustomClaimFields = []string{
	"account_id",
	"creator_email",
	"creator_id",
	"creator_idp",
	"creator_name",
	"creator_principal",
	"email",
	"environment_id",
	"environment_initializers.context_url",
	"environment_initializers.git.remote_uri",
	"environment_initializers.git.upstream_remote_uri",
	"idp",
	"name",
	"organization_id",
	"project_id",
	"runner_id",
	"runner_name",
	"service_account_id",
	"user_id",
}

func oidcConfigFromModel(ctx context.Context, data OIDCConfigModel) (*v1.OIDCConfig, diag.Diagnostics) {
	var diags diag.Diagnostics
	validateOIDCConfigModel(ctx, data, &diags)
	if diags.HasError() {
		return nil, diags
	}

	return &v1.OIDCConfig{
		Version: &v1.OIDCConfig_V3{
			V3: &v1.OIDCConfigV3{
				ExtraSubFields: stringSliceFromSet(ctx, data.CustomClaimFields, &diags),
			},
		},
	}, diags
}

func populateOIDCConfigModel(ctx context.Context, data *OIDCConfigModel, organizationID string, config *v1.OIDCConfig, prior OIDCConfigModel, diags *diag.Diagnostics) {
	data.ID = types.StringValue(organizationID)
	if config == nil {
		diags.AddError("Missing Ona OIDC Config", "The Ona API returned an empty OIDC config.")
		return
	}
	switch version := config.GetVersion().(type) {
	case *v1.OIDCConfig_V2:
		data.CustomClaimFields = types.SetNull(types.StringType)
		diags.AddError("Unsupported Ona OIDC Config Version", "The organization uses OIDC V2, but ona_oidc_config supports only OIDC V3. Upgrade the organization to OIDC V3 before managing it with Terraform.")
	case *v1.OIDCConfig_V3:
		data.CustomClaimFields = stringSetValue(ctx, sortedStrings(version.V3.GetExtraSubFields()), prior.CustomClaimFields, true, diags)
	default:
		data.CustomClaimFields = types.SetNull(types.StringType)
		diags.AddError("Unsupported Ona OIDC Config Version", "The Ona API returned an OIDC configuration without a supported V3 token format.")
	}
}

func preserveOIDCConfigPlannedInputs(data *OIDCConfigModel, planned OIDCConfigModel) {
	data.CustomClaimFields = preserveSet(data.CustomClaimFields, planned.CustomClaimFields)
}

func validateOIDCConfigModel(ctx context.Context, data OIDCConfigModel, diags *diag.Diagnostics) {
	if data.CustomClaimFields.IsNull() || data.CustomClaimFields.IsUnknown() {
		return
	}
	var customClaimFields []string
	diags.Append(data.CustomClaimFields.ElementsAs(ctx, &customClaimFields, false)...)
	if diags.HasError() {
		return
	}
	allowed := make(map[string]struct{}, len(oidcCustomClaimFields))
	for _, field := range oidcCustomClaimFields {
		allowed[field] = struct{}{}
	}
	var unsupported []string
	for _, field := range customClaimFields {
		if _, ok := allowed[field]; !ok {
			unsupported = append(unsupported, field)
		}
	}
	if len(unsupported) > 0 {
		sort.Strings(unsupported)
		diags.AddAttributeError(
			path.Root("custom_claim_fields"),
			"Unsupported OIDC Custom Claim Fields",
			fmt.Sprintf("Unsupported fields: %s. Supported fields are: %s.", strings.Join(unsupported, ", "), strings.Join(oidcCustomClaimFields, ", ")),
		)
	}
}
