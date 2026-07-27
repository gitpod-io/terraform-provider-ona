package main

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/zclconf/go-cty/cty"
)

func TestRewriteGeneratedConfigReferences(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result string
		Err    string
	}
	tests := []struct {
		Name     string
		Input    string
		Expected Expectation
	}{
		{
			Name: "identity_imports_drive_resource_references",
			Input: `import {
  to = ona_runner.frankfurt
  identity = {
    runner_id = "runner-1"
  }
}

import {
  to = ona_environment_class.large
  identity = {
    id = "class-1"
  }
}

import {
  to = ona_organization_role_assignment.admin
  identity = {
    group_id        = "group-1"
    organization_id = "org-1"
    role            = "admin"
  }
}

import {
  to       = ona_organization_policies.current
  identity = { organization_id = "org-1" }
}

resource "ona_environment_class" "large" {
  runner_id = "runner-1"
}

resource "ona_project" "next" {
  environment_class_ids = ["class-1", "external-class"]
  organization_id       = "org-1"
}
`,
			Expected: Expectation{Result: `import {
  to = ona_runner.frankfurt
  identity = {
    runner_id = "runner-1"
  }
}

import {
  to = ona_environment_class.large
  identity = {
    id = "class-1"
  }
}

import {
  to = ona_organization_role_assignment.admin
  identity = {
    group_id        = "group-1"
    organization_id = "org-1"
    role            = "admin"
  }
}

import {
  to       = ona_organization_policies.current
  identity = { organization_id = "org-1" }
}

resource "ona_environment_class" "large" {
  runner_id = ona_runner.frankfurt.id
}

resource "ona_project" "next" {
  environment_class_ids = [
    ona_environment_class.large.id,
    "external-class",
  ]
  organization_id = "org-1"
}
`},
		},
		{
			Name: "ambiguous_identity_is_not_rewritten",
			Input: `import {
  to       = ona_runner.one
  identity = { runner_id = "shared-id" }
}
import {
  to       = ona_runner.two
  identity = { runner_id = "shared-id" }
}
resource "ona_scm_integration" "test" {
  runner_id = "shared-id"
}
`,
			Expected: Expectation{Result: `import {
  to       = ona_runner.one
  identity = { runner_id = "shared-id" }
}
import {
  to       = ona_runner.two
  identity = { runner_id = "shared-id" }
}
resource "ona_scm_integration" "test" {
  runner_id = "shared-id"
}
`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "generated.tf")
			var got Expectation
			if err := os.WriteFile(path, []byte(tc.Input), 0644); err != nil {
				got.Err = err.Error()
			} else if err := rewriteGeneratedConfig(path); err != nil {
				got.Err = err.Error()
			} else if data, err := os.ReadFile(path); err != nil {
				got.Err = err.Error()
			} else {
				got.Result = string(data)
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("rewriteGeneratedConfig() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSingleStringIdentity(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		AttributeName string
		Result        string
		OK            bool
	}
	tests := []struct {
		Name     string
		Input    cty.Value
		Expected Expectation
	}{
		{
			Name:     "single_string",
			Input:    cty.ObjectVal(map[string]cty.Value{"id": cty.StringVal("resource-1")}),
			Expected: Expectation{AttributeName: "id", Result: "resource-1", OK: true},
		},
		{
			Name: "optional_null_is_ignored",
			Input: cty.ObjectVal(map[string]cty.Value{
				"id":    cty.StringVal("resource-1"),
				"scope": cty.NullVal(cty.String),
			}),
			Expected: Expectation{AttributeName: "id", Result: "resource-1", OK: true},
		},
		{
			Name: "composite_identity",
			Input: cty.ObjectVal(map[string]cty.Value{
				"id":    cty.StringVal("resource-1"),
				"scope": cty.StringVal("organization-1"),
			}),
			Expected: Expectation{},
		},
		{
			Name:     "non_string_identity",
			Input:    cty.ObjectVal(map[string]cty.Value{"id": cty.NumberIntVal(1)}),
			Expected: Expectation{},
		},
		{
			Name:     "tuple_is_not_an_identity_object",
			Input:    cty.TupleVal([]cty.Value{cty.StringVal("resource-1")}),
			Expected: Expectation{},
		},
		{
			Name:     "unknown_identity",
			Input:    cty.UnknownVal(cty.Object(map[string]cty.Type{"id": cty.String})),
			Expected: Expectation{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			attributeName, result, ok := singleStringIdentity(tc.Input)
			got := Expectation{AttributeName: attributeName, Result: result, OK: ok}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("singleStringIdentity() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestIdentityMatchesResourceType(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Result bool
	}
	tests := []struct {
		Name              string
		ResourceType      string
		IdentityAttribute string
		Expected          Expectation
	}{
		{Name: "generic_id", ResourceType: "ona_scm_integration", IdentityAttribute: "id", Expected: Expectation{Result: true}},
		{Name: "resource_specific_id", ResourceType: "ona_runner", IdentityAttribute: "runner_id", Expected: Expectation{Result: true}},
		{Name: "organization_singleton", ResourceType: "ona_organization_policies", IdentityAttribute: "organization_id", Expected: Expectation{}},
		{Name: "unrelated_id", ResourceType: "ona_project", IdentityAttribute: "runner_id", Expected: Expectation{}},
		{Name: "non_ona_resource", ResourceType: "example_widget", IdentityAttribute: "widget_id", Expected: Expectation{}},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			got := Expectation{Result: identityMatchesResourceType(tc.ResourceType, tc.IdentityAttribute)}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("identityMatchesResourceType() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSplitGeneratedConfigByBlockType(t *testing.T) {
	t.Parallel()

	input := `import {
  to       = ona_runner.frankfurt
  identity = { runner_id = "runner-1" }
}

resource "ona_runner" "frankfurt" {
  name = "Frankfurt"
}

resource "ona_scm_integration" "github" {
  host = "github.com"
}

locals {
  note = "review generated configuration"
}
`
	dir := t.TempDir()
	path := filepath.Join(dir, "generated.tf")
	if err := os.WriteFile(path, []byte(input), 0644); err != nil {
		t.Fatalf("write generated fixture: %v", err)
	}

	type Expectation struct {
		Files []string
		Err   string
	}
	var got Expectation
	paths, err := splitGeneratedConfig(path, dir)
	if err != nil {
		got.Err = err.Error()
	} else {
		for _, resultPath := range paths {
			got.Files = append(got.Files, filepath.Base(resultPath))
		}
		sort.Strings(got.Files)
	}

	expected := Expectation{Files: []string{"generated_misc.tf", "imports.tf", "runners.tf", "scm_integration.tf"}}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("splitGeneratedConfig() mismatch (-want +got):\n%s", diff)
	}
}
