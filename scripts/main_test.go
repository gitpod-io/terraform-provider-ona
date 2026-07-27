package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestRunRejectsMissingQueryFile(t *testing.T) {
	t.Parallel()

	queryPath := filepath.Join(t.TempDir(), "missing.tfquery.hcl")
	type Expectation struct {
		Err string
	}
	var got Expectation
	if err := run(config{queryFile: queryPath}); err != nil {
		got.Err = err.Error()
	}

	expected := Expectation{Err: "read query configuration " + queryPath + ": open " + queryPath + ": no such file or directory"}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("run() mismatch (-want +got):\n%s", diff)
	}
}

func TestRunPostProcessesTerraformQueryOutput(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	queryPath := filepath.Join(tempDir, "source.tfquery.hcl")
	if err := os.WriteFile(queryPath, []byte(`list "ona_runner" "all" {
  provider         = ona
  include_resource = true
}
`), 0644); err != nil {
		t.Fatalf("write query fixture: %v", err)
	}

	generatedPath := filepath.Join(tempDir, "query-output.tf")
	if err := os.WriteFile(generatedPath, []byte(`import {
  to       = ona_runner.frankfurt
  identity = { runner_id = "runner-1" }
}
import {
  to       = ona_scm_integration.github
  identity = { id = "scm-1" }
}
resource "ona_runner" "frankfurt" {
  name            = "Frankfurt"
  runner_provider = "aws_ec2"
}
resource "ona_scm_integration" "github" {
  runner_id = "runner-1"
  host      = "github.com"
}
`), 0644); err != nil {
		t.Fatalf("write generated fixture: %v", err)
	}

	workdir := filepath.Join(tempDir, "work")
	terraformPath := filepath.Join(tempDir, "terraform")
	fakeTerraform := fmt.Sprintf(`#!/bin/sh
set -eu
command_name="$2"
case "$command_name" in
  query)
    cp %q %q
    ;;
  fmt|validate)
    ;;
  plan)
    touch %q
    exit 2
    ;;
  show)
    printf '%%s\n' '{"resource_changes":[{"address":"ona_runner.frankfurt","change":{"actions":["no-op"]}},{"address":"ona_scm_integration.github","change":{"actions":["no-op"]}}]}'
    ;;
  *)
    echo "unexpected terraform command: $command_name" >&2
    exit 1
    ;;
esac
`, generatedPath, filepath.Join(workdir, "generated.tf"), filepath.Join(workdir, "validation.tfplan"))
	if err := os.WriteFile(terraformPath, []byte(fakeTerraform), 0755); err != nil {
		t.Fatalf("write terraform fixture: %v", err)
	}

	cfg := config{
		host:                 "https://example.com",
		token:                "test-token",
		queryFile:            queryPath,
		workdir:              workdir,
		providerDir:          "..",
		terraform:            terraformPath,
		terraformParallelism: 2,
	}

	type Expectation struct {
		SCMConfig string
		Files     []string
		Err       string
	}
	var got Expectation
	if err := run(cfg); err != nil {
		got.Err = err.Error()
	} else if data, err := os.ReadFile(filepath.Join(workdir, "scm_integration.tf")); err != nil {
		got.Err = err.Error()
	} else if entries, err := os.ReadDir(workdir); err != nil {
		got.Err = err.Error()
	} else {
		got.SCMConfig = string(data)
		for _, entry := range entries {
			if !entry.IsDir() {
				got.Files = append(got.Files, entry.Name())
			}
		}
	}

	expected := Expectation{
		SCMConfig: `resource "ona_scm_integration" "github" {
  runner_id = ona_runner.frankfurt.id
  host      = "github.com"
}

`,
		Files: []string{
			"generated.raw.tf.txt",
			"generated.rewritten.tf.txt",
			"imports.tf",
			"provider.tf",
			"query.tfquery.hcl",
			"runners.tf",
			"scm_integration.tf",
			"terraformrc",
			"versions.tf",
		},
	}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("run() mismatch (-want +got):\n%s", diff)
	}
}
