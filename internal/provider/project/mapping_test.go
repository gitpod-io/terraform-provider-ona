// Copyright IBM Corp. 2021, 2026
// SPDX-License-Identifier: MPL-2.0

package project

import (
	"testing"
	"time"

	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/google/go-cmp/cmp"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
)

func TestEnvironmentInitializerFromRepository(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result *v1.EnvironmentInitializer
	}

	tests := []struct {
		Name     string
		Input    ProjectModel
		Expected Expectation
	}{
		{
			Name: "uses_repository_clone_url_and_branch",
			Input: ProjectModel{
				RepositoryCloneURL: types.StringValue("https://github.com/gitpod-io/gitpod-next.git"),
				Branch:             types.StringValue("main"),
			},
			Expected: Expectation{
				Result: &v1.EnvironmentInitializer{
					Specs: []*v1.EnvironmentInitializer_Spec{
						{
							Spec: &v1.EnvironmentInitializer_Spec_Git{
								Git: &v1.GitInitializer{
									RemoteUri:   "https://github.com/gitpod-io/gitpod-next.git",
									TargetMode:  v1.GitInitializer_CLONE_TARGET_MODE_REMOTE_BRANCH,
									CloneTarget: "main",
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			got := Expectation{
				Result: environmentInitializerFromRepository(tc.Input),
			}

			if diff := cmp.Diff(tc.Expected, got, protocmp.Transform()); diff != "" {
				t.Errorf("environmentInitializerFromRepository() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRepositoryFromInitializer(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result repositoryFields
		Err    string
	}

	tests := []struct {
		Name     string
		Input    *v1.EnvironmentInitializer
		Expected Expectation
	}{
		{
			Name: "extracts_git_repository_fields",
			Input: &v1.EnvironmentInitializer{
				Specs: []*v1.EnvironmentInitializer_Spec{
					{
						Spec: &v1.EnvironmentInitializer_Spec_Git{
							Git: &v1.GitInitializer{
								RemoteUri:   "https://github.com/gitpod-io/gitpod-next.git",
								TargetMode:  v1.GitInitializer_CLONE_TARGET_MODE_REMOTE_BRANCH,
								CloneTarget: "main",
							},
						},
					},
				},
			},
			Expected: Expectation{
				Result: repositoryFields{
					CloneURL: types.StringValue("https://github.com/gitpod-io/gitpod-next.git"),
					Branch:   types.StringValue("main"),
				},
			},
		},
		{
			Name: "rejects_context_url_initializer",
			Input: &v1.EnvironmentInitializer{
				Specs: []*v1.EnvironmentInitializer_Spec{
					{
						Spec: &v1.EnvironmentInitializer_Spec_ContextUrl{
							ContextUrl: &v1.ContextURLInitializer{Url: "https://github.com/gitpod-io/gitpod-next"},
						},
					},
				},
			},
			Expected: Expectation{
				Err: "Unsupported Ona Project Repository",
			},
		},
		{
			Name: "rejects_missing_branch",
			Input: &v1.EnvironmentInitializer{
				Specs: []*v1.EnvironmentInitializer_Spec{
					{
						Spec: &v1.EnvironmentInitializer_Spec_Git{
							Git: &v1.GitInitializer{
								RemoteUri:  "https://github.com/gitpod-io/gitpod-next.git",
								TargetMode: v1.GitInitializer_CLONE_TARGET_MODE_REMOTE_HEAD,
							},
						},
					},
				},
			},
			Expected: Expectation{
				Err: "Unsupported Ona Project Repository",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			result, diags := repositoryFromInitializer(tc.Input)
			if diags.HasError() {
				got.Err = diags[0].Summary()
			} else {
				got.Result = result
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("repositoryFromInitializer() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestProjectEnvironmentClassesFromModel(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result []*v1.ProjectEnvironmentClass
		Err    string
	}

	tests := []struct {
		Name     string
		Input    []EnvironmentClassModel
		Expected Expectation
	}{
		{
			Name: "fixed_and_local_runner_sorted_by_order",
			Input: []EnvironmentClassModel{
				{
					LocalRunner: types.BoolValue(true),
					Order:       types.Int64Value(1),
				},
				{
					EnvironmentClassID: types.StringValue("class-1"),
					Order:              types.Int64Value(0),
				},
			},
			Expected: Expectation{
				Result: []*v1.ProjectEnvironmentClass{
					{
						EnvironmentClass: &v1.ProjectEnvironmentClass_EnvironmentClassId{EnvironmentClassId: "class-1"},
						Order:            0,
					},
					{
						EnvironmentClass: &v1.ProjectEnvironmentClass_LocalRunner{LocalRunner: true},
						Order:            1,
					},
				},
			},
		},
		{
			Name: "rejects_missing_oneof",
			Input: []EnvironmentClassModel{
				{Order: types.Int64Value(0)},
			},
			Expected: Expectation{
				Err: "Invalid Project Environment Class",
			},
		},
		{
			Name: "rejects_duplicate_order",
			Input: []EnvironmentClassModel{
				{
					EnvironmentClassID: types.StringValue("class-1"),
					Order:              types.Int64Value(0),
				},
				{
					EnvironmentClassID: types.StringValue("class-2"),
					Order:              types.Int64Value(0),
				},
			},
			Expected: Expectation{
				Err: "Duplicate Project Environment Class Order",
			},
		},
		{
			Name: "rejects_duplicate_environment_class_id",
			Input: []EnvironmentClassModel{
				{
					EnvironmentClassID: types.StringValue("class-1"),
					Order:              types.Int64Value(0),
				},
				{
					EnvironmentClassID: types.StringValue("class-1"),
					Order:              types.Int64Value(1),
				},
			},
			Expected: Expectation{
				Err: "Duplicate Project Environment Class",
			},
		},
		{
			Name: "rejects_duplicate_local_runner",
			Input: []EnvironmentClassModel{
				{
					LocalRunner: types.BoolValue(true),
					Order:       types.Int64Value(0),
				},
				{
					LocalRunner: types.BoolValue(true),
					Order:       types.Int64Value(1),
				},
			},
			Expected: Expectation{
				Err: "Duplicate Local Runner Environment Class",
			},
		},
		{
			Name: "rejects_negative_order",
			Input: []EnvironmentClassModel{
				{
					EnvironmentClassID: types.StringValue("class-1"),
					Order:              types.Int64Value(-1),
				},
			},
			Expected: Expectation{
				Err: "Invalid Project Environment Class Order",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			result, diags := projectEnvironmentClassesFromModel(tc.Input, path.Root("environment_class"), false)
			if diags.HasError() {
				got.Err = diags[0].Summary()
			} else {
				got.Result = result
			}

			if diff := cmp.Diff(tc.Expected, got, protocmp.Transform()); diff != "" {
				t.Errorf("projectEnvironmentClassesFromModel() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestValidateProjectModel(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Err string
	}

	tests := []struct {
		Name     string
		Mutate   func(*ProjectModel)
		Expected Expectation
	}{
		{
			Name:     "accepts_https_clone_url_and_relative_paths",
			Expected: Expectation{},
		},
		{
			Name: "accepts_scp_like_ssh_clone_url",
			Mutate: func(input *ProjectModel) {
				input.RepositoryCloneURL = types.StringValue("git@github.com:ona/example.git")
			},
			Expected: Expectation{},
		},
		{
			Name: "rejects_bare_repository_value",
			Mutate: func(input *ProjectModel) {
				input.RepositoryCloneURL = types.StringValue("ona/example")
			},
			Expected: Expectation{
				Err: "Invalid Project Repository Clone URL",
			},
		},
		{
			Name: "rejects_blank_branch",
			Mutate: func(input *ProjectModel) {
				input.Branch = types.StringValue("  ")
			},
			Expected: Expectation{
				Err: "Missing Project Branch",
			},
		},
		{
			Name: "rejects_absolute_devcontainer_path",
			Mutate: func(input *ProjectModel) {
				input.DevcontainerFilePath = types.StringValue("/.devcontainer/devcontainer.json")
			},
			Expected: Expectation{
				Err: "Invalid Project File Path",
			},
		},
		{
			Name: "rejects_parent_directory_automations_path",
			Mutate: func(input *ProjectModel) {
				input.AutomationsFilePath = types.StringValue("../automations.yaml")
			},
			Expected: Expectation{
				Err: "Invalid Project File Path",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			input := validProjectModel()
			if tc.Mutate != nil {
				tc.Mutate(&input)
			}

			var diags diag.Diagnostics
			validateProjectModel(t.Context(), input, &diags)

			var got Expectation
			if diags.HasError() {
				got.Err = diags[0].Summary()
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("validateProjectModel() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func validProjectModel() ProjectModel {
	return ProjectModel{
		Name:                 types.StringValue("Example"),
		RepositoryCloneURL:   types.StringValue("https://github.com/ona/example.git"),
		Branch:               types.StringValue("main"),
		DevcontainerFilePath: types.StringValue(".devcontainer/devcontainer.json"),
		AutomationsFilePath:  types.StringValue(".ona/automations.yaml"),
		EnvironmentClasses: []EnvironmentClassModel{
			{
				EnvironmentClassID: types.StringValue("class-1"),
				Order:              types.Int64Value(0),
			},
		},
	}
}

func TestPrebuildConfigurationFromModel(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result *v1.ProjectPrebuildConfiguration
		Err    string
	}

	tests := []struct {
		Name     string
		Input    []PrebuildConfigurationModel
		Expected Expectation
	}{
		{
			Name: "daily_schedule_with_executor",
			Input: []PrebuildConfigurationModel{{
				Enabled: types.BoolValue(true),
				EnvironmentClassIDs: types.SetValueMust(types.StringType, []attr.Value{
					types.StringValue("class-1"),
				}),
				Timeout: types.StringValue("30m"),
				DailySchedule: []DailyScheduleModel{{
					HourUTC: types.Int64Value(3),
				}},
				Executor: []SubjectModel{{
					ID:        types.StringValue("executor-1"),
					Principal: types.StringValue(principalServiceAccount),
				}},
				EnableJetbrainsWarmup: types.BoolValue(true),
			}},
			Expected: Expectation{
				Result: &v1.ProjectPrebuildConfiguration{
					Enabled:             true,
					EnvironmentClassIds: []string{"class-1"},
					Timeout:             durationpb.New(30 * time.Minute),
					Trigger: &v1.PrebuildTrigger{
						Trigger: &v1.PrebuildTrigger_DailySchedule_{
							DailySchedule: &v1.PrebuildTrigger_DailySchedule{HourUtc: 3},
						},
					},
					Executor: &v1.Subject{
						Id:        "executor-1",
						Principal: v1.Principal_PRINCIPAL_SERVICE_ACCOUNT,
					},
					EnableJetbrainsWarmup: true,
				},
			},
		},
		{
			Name: "rejects_timeout_out_of_range",
			Input: []PrebuildConfigurationModel{{
				Enabled:               types.BoolValue(true),
				EnvironmentClassIDs:   types.SetNull(types.StringType),
				Timeout:               types.StringValue("3h"),
				EnableJetbrainsWarmup: types.BoolValue(false),
			}},
			Expected: Expectation{
				Err: "Invalid Prebuild Timeout",
			},
		},
		{
			Name: "uses_default_timeout_when_unknown",
			Input: []PrebuildConfigurationModel{{
				Enabled: types.BoolValue(true),
				EnvironmentClassIDs: types.SetValueMust(types.StringType, []attr.Value{
					types.StringValue("class-1"),
				}),
				Timeout: types.StringUnknown(),
			}},
			Expected: Expectation{
				Result: &v1.ProjectPrebuildConfiguration{
					Enabled:             true,
					EnvironmentClassIds: []string{"class-1"},
					Timeout:             durationpb.New(time.Hour),
				},
			},
		},
		{
			Name: "rejects_daily_schedule_hour_out_of_range",
			Input: []PrebuildConfigurationModel{{
				Enabled:               types.BoolValue(true),
				EnvironmentClassIDs:   types.SetNull(types.StringType),
				Timeout:               types.StringValue("1h"),
				EnableJetbrainsWarmup: types.BoolValue(false),
				DailySchedule: []DailyScheduleModel{{
					HourUTC: types.Int64Value(24),
				}},
			}},
			Expected: Expectation{
				Err: "Invalid Daily Schedule Hour",
			},
		},
		{
			Name: "rejects_missing_executor_id",
			Input: []PrebuildConfigurationModel{{
				Enabled:               types.BoolValue(true),
				EnvironmentClassIDs:   types.SetNull(types.StringType),
				Timeout:               types.StringValue("1h"),
				EnableJetbrainsWarmup: types.BoolValue(false),
				Executor: []SubjectModel{{
					ID:        types.StringValue(""),
					Principal: types.StringValue(principalServiceAccount),
				}},
			}},
			Expected: Expectation{
				Err: "Missing Prebuild Executor ID",
			},
		},
		{
			Name: "rejects_invalid_executor_principal",
			Input: []PrebuildConfigurationModel{{
				Enabled:               types.BoolValue(true),
				EnvironmentClassIDs:   types.SetNull(types.StringType),
				Timeout:               types.StringValue("1h"),
				EnableJetbrainsWarmup: types.BoolValue(false),
				Executor: []SubjectModel{{
					ID:        types.StringValue("executor-1"),
					Principal: types.StringValue("runner"),
				}},
			}},
			Expected: Expectation{
				Err: "Invalid Prebuild Executor Principal",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			result, diags := prebuildConfigurationFromModel(t.Context(), tc.Input, path.Root("prebuild_configuration"))
			if diags.HasError() {
				got.Err = diags[0].Summary()
			} else {
				got.Result = result
			}

			if diff := cmp.Diff(tc.Expected, got, protocmp.Transform()); diff != "" {
				t.Errorf("prebuildConfigurationFromModel() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestPreserveProjectPlannedInputs(t *testing.T) {
	t.Parallel()

	type Input struct {
		Current ProjectModel
		Planned ProjectModel
	}
	type Expectation struct {
		Result ProjectModel
	}

	tests := []struct {
		Name     string
		Input    Input
		Expected Expectation
	}{
		{
			Name: "repository_fields_preserve_known_planned_values",
			Input: Input{
				Current: ProjectModel{
					Name:               types.StringValue("terraform-provider-devloop"),
					RepositoryCloneURL: types.StringValue("https://github.com/gitpod-io/gitpod-next.git"),
					Branch:             types.StringValue("main"),
				},
				Planned: ProjectModel{
					Name:               types.StringValue("Terraform Provider Devloop"),
					RepositoryCloneURL: types.StringValue("https://github.com/gitpod-io/gitpod.git"),
					Branch:             types.StringValue("stable"),
				},
			},
			Expected: Expectation{
				Result: ProjectModel{
					Name:               types.StringValue("Terraform Provider Devloop"),
					RepositoryCloneURL: types.StringValue("https://github.com/gitpod-io/gitpod.git"),
					Branch:             types.StringValue("stable"),
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			got := Expectation{
				Result: tc.Input.Current,
			}
			preserveProjectPlannedInputs(&got.Result, tc.Input.Planned)

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("preserveProjectPlannedInputs() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
