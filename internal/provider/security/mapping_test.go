// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package security

import (
	"testing"

	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestAdmissionLevelMappings(t *testing.T) {
	tests := []struct {
		name  string
		value string
		enum  v1.AdmissionLevel
	}{
		{name: "everyone", value: admissionLevelEveryone, enum: v1.AdmissionLevel_ADMISSION_LEVEL_EVERYONE},
		{name: "organization", value: admissionLevelOrganization, enum: v1.AdmissionLevel_ADMISSION_LEVEL_ORGANIZATION},
		{name: "creator only", value: admissionLevelCreatorOnly, enum: v1.AdmissionLevel_ADMISSION_LEVEL_CREATOR_ONLY},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var diags diag.Diagnostics
			if diff := cmp.Diff(test.enum, admissionLevelFromString(types.StringValue(test.value), path.Root("max_admission_level"), &diags)); diff != "" {
				t.Fatalf("enum mismatch (-want +got):\n%s", diff)
			}
			if diags.HasError() {
				t.Fatalf("mapping string to enum: %v", diags)
			}
			if diff := cmp.Diff(test.value, admissionLevelToString(test.enum).ValueString()); diff != "" {
				t.Fatalf("string mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAdmissionLevelFromStringRejectsUnsupportedValue(t *testing.T) {
	var diags diag.Diagnostics
	got := admissionLevelFromString(types.StringValue("public"), path.Root("max_admission_level"), &diags)
	if diff := cmp.Diff(v1.AdmissionLevel_ADMISSION_LEVEL_UNSPECIFIED, got); diff != "" {
		t.Fatalf("enum mismatch (-want +got):\n%s", diff)
	}
	if !diags.HasError() {
		t.Fatal("expected an invalid admission level diagnostic")
	}
}
