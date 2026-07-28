// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package project

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func TestProjectDataSourceModel(t *testing.T) {
	t.Parallel()

	creator := types.ObjectValueMust(subjectObjectAttributeTypes, map[string]attr.Value{
		"id":        types.StringValue("user-1"),
		"principal": types.StringValue(principalUser),
	})
	prebuildClassIDs := types.SetValueMust(types.StringType, []attr.Value{types.StringValue("class-1")})

	tests := []struct {
		Name     string
		Input    ProjectModel
		Expected ProjectDataSourceModel
	}{
		{
			Name: "copies_managed_project_fields_and_sets_lookup_id",
			Input: ProjectModel{
				ID:                   types.StringValue("project-1"),
				Name:                 types.StringValue("Acme API"),
				RepositoryCloneURL:   types.StringValue("https://github.com/acme/api.git"),
				Branch:               types.StringValue("main"),
				InsightsEnabled:      types.BoolValue(true),
				DevcontainerFilePath: types.StringValue(".devcontainer/devcontainer.json"),
				AutomationsFilePath:  types.StringValue(".ona/automations.yaml"),
				EnvironmentClasses: []EnvironmentClassModel{{
					EnvironmentClassID: types.StringValue("class-1"),
					LocalRunner:        types.BoolNull(),
					Order:              types.Int64Value(0),
				}},
				Prebuild: []PrebuildConfigurationModel{{
					Enabled:               types.BoolValue(true),
					EnvironmentClassIDs:   prebuildClassIDs,
					Timeout:               types.StringValue("30m0s"),
					DailySchedule:         []DailyScheduleModel{{HourUTC: types.Int64Value(3)}},
					Executor:              []SubjectModel{{ID: types.StringValue("service-account-1"), Principal: types.StringValue(principalServiceAccount)}},
					EnableJetbrainsWarmup: types.BoolValue(true),
				}},
				CreatedAt: types.StringValue("2026-07-07T12:00:00Z"),
				Creator:   creator,
			},
			Expected: ProjectDataSourceModel{
				ID:                   types.StringValue("project-1"),
				ProjectID:            types.StringValue("project-1"),
				Name:                 types.StringValue("Acme API"),
				RepositoryCloneURL:   types.StringValue("https://github.com/acme/api.git"),
				Branch:               types.StringValue("main"),
				InsightsEnabled:      types.BoolValue(true),
				DevcontainerFilePath: types.StringValue(".devcontainer/devcontainer.json"),
				AutomationsFilePath:  types.StringValue(".ona/automations.yaml"),
				EnvironmentClasses: []EnvironmentClassModel{{
					EnvironmentClassID: types.StringValue("class-1"),
					LocalRunner:        types.BoolNull(),
					Order:              types.Int64Value(0),
				}},
				Prebuild: []PrebuildConfigurationModel{{
					Enabled:               types.BoolValue(true),
					EnvironmentClassIDs:   prebuildClassIDs,
					Timeout:               types.StringValue("30m0s"),
					DailySchedule:         []DailyScheduleModel{{HourUTC: types.Int64Value(3)}},
					Executor:              []SubjectModel{{ID: types.StringValue("service-account-1"), Principal: types.StringValue(principalServiceAccount)}},
					EnableJetbrainsWarmup: types.BoolValue(true),
				}},
				CreatedAt: types.StringValue("2026-07-07T12:00:00Z"),
				Creator:   creator,
			},
		},
		{
			Name: "preserves_null_optional_values",
			Input: ProjectModel{
				ID:                   types.StringValue("project-2"),
				DevcontainerFilePath: types.StringNull(),
				AutomationsFilePath:  types.StringNull(),
				CreatedAt:            types.StringNull(),
				Creator:              types.ObjectNull(subjectObjectAttributeTypes),
			},
			Expected: ProjectDataSourceModel{
				ID:                   types.StringValue("project-2"),
				ProjectID:            types.StringValue("project-2"),
				DevcontainerFilePath: types.StringNull(),
				AutomationsFilePath:  types.StringNull(),
				CreatedAt:            types.StringNull(),
				Creator:              types.ObjectNull(subjectObjectAttributeTypes),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			got := projectDataSourceModel(tc.Input)
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("projectDataSourceModel() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
