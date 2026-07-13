// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package organization

import (
	"context"

	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const (
	oidcVersionV2 = "v2"
	oidcVersionV3 = "v3"
)

func oidcConfigFromModel(ctx context.Context, data OIDCConfigModel) (*v1.OIDCConfig, diag.Diagnostics) {
	var diags diag.Diagnostics
	validateOIDCConfigModel(ctx, data, &diags)
	if diags.HasError() {
		return nil, diags
	}

	switch data.Version.ValueString() {
	case oidcVersionV2:
		return &v1.OIDCConfig{
			Version: &v1.OIDCConfig_V2{V2: &v1.OIDCConfigV2{}},
		}, diags
	case oidcVersionV3:
		return &v1.OIDCConfig{
			Version: &v1.OIDCConfig_V3{
				V3: &v1.OIDCConfigV3{
					ExtraSubFields: stringSliceFromSet(ctx, data.ExtraSubFields, &diags),
				},
			},
		}, diags
	default:
		diags.AddAttributeError(path.Root("version"), "Invalid OIDC Config Version", "Supported values are \"v2\" and \"v3\".")
		return nil, diags
	}
}

func populateOIDCConfigModel(ctx context.Context, data *OIDCConfigModel, organizationID string, config *v1.OIDCConfig, prior OIDCConfigModel, diags *diag.Diagnostics) {
	data.ID = types.StringValue(organizationID)
	if config == nil {
		diags.AddError("Missing Ona OIDC Config", "The Ona API returned an empty OIDC config.")
		return
	}
	switch version := config.GetVersion().(type) {
	case *v1.OIDCConfig_V2:
		data.Version = types.StringValue(oidcVersionV2)
		data.ExtraSubFields = stringSetValue(ctx, nil, prior.ExtraSubFields, true, diags)
	case *v1.OIDCConfig_V3:
		data.Version = types.StringValue(oidcVersionV3)
		data.ExtraSubFields = stringSetValue(ctx, sortedStrings(version.V3.GetExtraSubFields()), prior.ExtraSubFields, true, diags)
	default:
		data.Version = types.StringNull()
		data.ExtraSubFields = types.SetNull(types.StringType)
	}
}

func preserveOIDCConfigPlannedInputs(data *OIDCConfigModel, planned OIDCConfigModel) {
	data.Version = preserveString(data.Version, planned.Version)
	data.ExtraSubFields = preserveSet(data.ExtraSubFields, planned.ExtraSubFields)
}

func validateOIDCConfigModel(ctx context.Context, data OIDCConfigModel, diags *diag.Diagnostics) {
	if isKnownString(data.Version) {
		switch data.Version.ValueString() {
		case oidcVersionV2:
			if !data.ExtraSubFields.IsNull() && !data.ExtraSubFields.IsUnknown() && len(data.ExtraSubFields.Elements()) > 0 {
				diags.AddAttributeError(path.Root("extra_sub_fields"), "Invalid OIDC V2 Extra Subject Fields", "extra_sub_fields is only supported when version is \"v3\".")
			}
		case oidcVersionV3:
		default:
			diags.AddAttributeError(path.Root("version"), "Invalid OIDC Config Version", "Supported values are \"v2\" and \"v3\".")
		}
	}
	var extraSubFields []string
	if !data.ExtraSubFields.IsNull() && !data.ExtraSubFields.IsUnknown() {
		diags.Append(data.ExtraSubFields.ElementsAs(ctx, &extraSubFields, false)...)
	}
}
