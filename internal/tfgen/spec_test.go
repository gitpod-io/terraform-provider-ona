// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package tfgen

import (
	"testing"

	apiterraform "github.com/gitpod-io/terraform-provider-ona/internal/api/go/tools/terraform"
	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/google/go-cmp/cmp"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func TestBuildSpecificationFromRunnerAnnotations(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		JSON string
		Err  string
	}

	var got Expectation
	specification, err := BuildSpecification(v1.File_gitpod_v1_runner_proto)
	if err != nil {
		got.Err = err.Error()
	} else {
		out, err := MarshalSpecification(specification)
		if err != nil {
			got.Err = err.Error()
		} else {
			got.JSON = string(out)
		}
	}

	expected := Expectation{
		JSON: `{
  "provider": {
    "name": "ona"
  },
  "resources": [
    {
      "name": "runner",
      "schema": {
        "attributes": [
          {
            "name": "id",
            "string": {
              "computed_optional_required": "computed",
              "description": "Runner ID. Use this value as the Terraform import ID.",
              "plan_modifiers": [
                {
                  "custom": {
                    "imports": [
                      {
                        "path": "github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
                      }
                    ],
                    "schema_definition": "stringplanmodifier.UseStateForUnknown()"
                  }
                }
              ]
            }
          },
          {
            "name": "name",
            "string": {
              "computed_optional_required": "computed",
              "description": "Runner name."
            }
          }
        ],
        "description": "Represents an Ona runner."
      }
    }
  ],
  "version": "0.1"
}
`,
	}

	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("BuildSpecification() mismatch (-want +got):\n%s", diff)
	}
}

func TestBuildResourceValidatesImportIDField(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Err string
	}

	tests := []struct {
		Name     string
		ImportID string
		Expected Expectation
	}{
		{
			Name:     "default_import_id_field",
			ImportID: "",
			Expected: Expectation{},
		},
		{
			Name:     "missing_proto_field",
			ImportID: "missing",
			Expected: Expectation{
				Err: `gitpod.v1.Runner Terraform import_id_field "missing" does not match a proto field`,
			},
		},
		{
			Name:     "field_without_terraform_attribute",
			ImportID: "created_at",
			Expected: Expectation{
				Err: `gitpod.v1.Runner Terraform import_id_field "created_at" must reference a field annotated with terraform_field`,
			},
		},
		{
			Name:     "annotated_proto_field",
			ImportID: "runner_id",
			Expected: Expectation{},
		},
	}

	runner := v1.File_gitpod_v1_runner_proto.Messages().ByName("Runner")
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var got Expectation
			_, err := buildResource(runner, &apiterraform.TerraformResource{
				ImportIdField: tc.ImportID,
			})
			if err != nil {
				got.Err = err.Error()
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("buildResource() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestStringPlanModifiersUseStateForUnknownDefaults(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Enabled bool
	}

	enabled := true
	disabled := false
	tests := []struct {
		Name          string
		FieldName     string
		ImportIDField string
		Annotation    *apiterraform.TerraformField
		Expected      Expectation
	}{
		{
			Name:          "import_id_field_enabled_by_default",
			FieldName:     "runner_id",
			ImportIDField: "runner_id",
			Annotation:    &apiterraform.TerraformField{},
			Expected: Expectation{
				Enabled: true,
			},
		},
		{
			Name:          "regular_field_disabled_by_default",
			FieldName:     "name",
			ImportIDField: "runner_id",
			Annotation:    &apiterraform.TerraformField{},
			Expected:      Expectation{},
		},
		{
			Name:          "import_id_field_can_disable_default",
			FieldName:     "runner_id",
			ImportIDField: "runner_id",
			Annotation: &apiterraform.TerraformField{
				UseStateForUnknown: &disabled,
			},
			Expected: Expectation{},
		},
		{
			Name:          "regular_field_can_enable_override",
			FieldName:     "name",
			ImportIDField: "runner_id",
			Annotation: &apiterraform.TerraformField{
				UseStateForUnknown: &enabled,
			},
			Expected: Expectation{
				Enabled: true,
			},
		},
	}

	runner := v1.File_gitpod_v1_runner_proto.Messages().ByName("Runner")
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			field := runner.Fields().ByName(protoreflect.Name(tc.FieldName))
			got := Expectation{
				Enabled: len(stringPlanModifiers(field, tc.ImportIDField, tc.Annotation)) > 0,
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("stringPlanModifiers() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
