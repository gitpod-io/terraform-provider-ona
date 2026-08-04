// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package project

import (
	"testing"

	v1 "github.com/gitpod-io/terraform-provider-ona/api/public-clients/go/v1"
	"github.com/gitpod-io/terraform-provider-ona/internal/provider/listutil"
	"github.com/google/go-cmp/cmp"
)

func TestProjectDisplayNames(t *testing.T) {
	t.Parallel()

	type Input struct {
		Projects []*v1.Project
	}
	type Expectation struct {
		Result []string
	}

	tests := []struct {
		Name     string
		Input    Input
		Expected Expectation
	}{
		{
			Name: "normalizes_project_names_as_terraform_labels",
			Input: Input{Projects: []*v1.Project{
				{Id: "project-1", Metadata: &v1.ProjectMetadata{Name: "  My Project!  "}},
				{Id: "project-2", Metadata: &v1.ProjectMetadata{Name: "123 Start"}},
			}},
			Expected: Expectation{Result: []string{"my_project", "r_123_start"}},
		},
		{
			Name: "falls_back_to_the_truncated_id_label",
			Input: Input{Projects: []*v1.Project{
				{Id: "abcdef12-3456-7890", Metadata: &v1.ProjectMetadata{Name: "!!!"}},
			}},
			Expected: Expectation{Result: []string{"r_abcdef12"}},
		},
		{
			Name: "uses_the_full_id_when_metadata_is_absent",
			Input: Input{Projects: []*v1.Project{
				{Id: "project-1"},
			}},
			Expected: Expectation{Result: []string{"project_1"}},
		},
		{
			Name: "suffixes_duplicate_labels_in_order",
			Input: Input{Projects: []*v1.Project{
				{Id: "project-1", Metadata: &v1.ProjectMetadata{Name: "Example"}},
				{Id: "project-2", Metadata: &v1.ProjectMetadata{Name: "Example"}},
				{Id: "project-3", Metadata: &v1.ProjectMetadata{Name: "Example"}},
			}},
			Expected: Expectation{Result: []string{"example", "example_2", "example_3"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			displayNames := listutil.NewDisplayNames()
			got := Expectation{Result: make([]string, 0, len(tc.Input.Projects))}
			for _, project := range tc.Input.Projects {
				got.Result = append(got.Result, projectDisplayName(displayNames, project))
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("projectDisplayName() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
