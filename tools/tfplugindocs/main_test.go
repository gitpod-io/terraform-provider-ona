// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRun(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Output string
		Err    string
	}

	tests := []struct {
		Name      string
		Output    string
		RunnerErr error
		Expected  Expectation
	}{
		{
			Name: "no_missing_templates",
			Output: "generating missing resource content\n" +
				"resource \"ona_project\" template exists, skipping\n" +
				"rendering static website\n",
			Expected: Expectation{
				Output: "generating missing resource content\n" +
					"resource \"ona_project\" template exists, skipping\n" +
					"rendering static website\n",
			},
		},
		{
			Name: "missing_resource_template",
			Output: "generating missing resource content\n" +
				"resource \"ona_skill\" fallback template exists, creating template\n",
			Expected: Expectation{
				Output: "generating missing resource content\n" +
					"resource \"ona_skill\" fallback template exists, creating template\n",
				Err: "tfplugindocs generated fallback templates; add checked-in templates for:\n" +
					"- resource \"ona_skill\"",
			},
		},
		{
			Name: "missing_multiple_component_kinds",
			Output: "generating missing resource content\n" +
				"generating new template for \"ona_skill\"\n" +
				"generating missing data source content\n" +
				"generating new template for data-source \"ona_skill\"\n" +
				"generating missing function content\n" +
				"generating new template for function \"ona_slug\"\n" +
				"generating missing ephemeral resource content\n" +
				"generating new template for \"ona_token\"\n" +
				"generating missing action content\n" +
				"generating new template for \"ona_restart\"\n" +
				"generating missing list resource content\n" +
				"generating new template for \"ona_skill\"\n" +
				"generating missing state store content\n" +
				"generating new template for \"ona_state\"\n" +
				"generating missing provider content\n" +
				"generating new template for \"ona\"\n",
			Expected: Expectation{
				Output: "generating missing resource content\n" +
					"generating new template for \"ona_skill\"\n" +
					"generating missing data source content\n" +
					"generating new template for data-source \"ona_skill\"\n" +
					"generating missing function content\n" +
					"generating new template for function \"ona_slug\"\n" +
					"generating missing ephemeral resource content\n" +
					"generating new template for \"ona_token\"\n" +
					"generating missing action content\n" +
					"generating new template for \"ona_restart\"\n" +
					"generating missing list resource content\n" +
					"generating new template for \"ona_skill\"\n" +
					"generating missing state store content\n" +
					"generating new template for \"ona_state\"\n" +
					"generating missing provider content\n" +
					"generating new template for \"ona\"\n",
				Err: "tfplugindocs generated fallback templates; add checked-in templates for:\n" +
					"- action \"ona_restart\"\n" +
					"- data source \"ona_skill\"\n" +
					"- ephemeral resource \"ona_token\"\n" +
					"- function \"ona_slug\"\n" +
					"- list resource \"ona_skill\"\n" +
					"- provider \"ona\"\n" +
					"- resource \"ona_skill\"\n" +
					"- state store \"ona_state\"",
			},
		},
		{
			Name: "unrelated_output",
			Output: "generating missing resource content\n" +
				"resource \"ona_project\" template exists, skipping\n" +
				"rendering static website\n" +
				"generating new template for \"unrelated\"\n",
			Expected: Expectation{
				Output: "generating missing resource content\n" +
					"resource \"ona_project\" template exists, skipping\n" +
					"rendering static website\n" +
					"generating new template for \"unrelated\"\n",
			},
		},
		{
			Name:      "generator_failure",
			Output:    "generating missing resource content\n",
			RunnerErr: errors.New("exit status 2"),
			Expected: Expectation{
				Output: "generating missing resource content\n",
				Err:    "tfplugindocs failed: exit status 2",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			var output bytes.Buffer
			runner := func(_ context.Context, stdout, _ io.Writer) error {
				_, _ = io.WriteString(stdout, tc.Output)
				return tc.RunnerErr
			}

			var got Expectation
			err := run(t.Context(), &output, io.Discard, runner)
			got.Output = output.String()
			if err != nil {
				got.Err = err.Error()
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("run() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
