package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestPrepareWorkdirRemovesGeneratedFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	files := map[string]string{
		"projects.tf":          "resource",
		"generated.raw.tf.txt": "raw",
		"terraformrc":          "rc",
		"terraform.sh":         "wrapper",
		".terraform.lock.hcl":  "lock",
		"validation.tfplan":    "plan",
		"plan":                 "saved plan",
		"query.tfquery.hcl":    "list",
	}
	for name, contents := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0644); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, ".terraform"), 0755); err != nil {
		t.Fatalf("create .terraform: %v", err)
	}
	if err := os.Mkdir(filepath.Join(dir, "keepdir"), 0755); err != nil {
		t.Fatalf("create keepdir: %v", err)
	}

	type Expectation struct {
		Files []string
		Err   string
	}
	var got Expectation
	if err := prepareWorkdir(dir); err != nil {
		got.Err = err.Error()
	} else if entries, err := os.ReadDir(dir); err != nil {
		got.Err = err.Error()
	} else {
		for _, entry := range entries {
			got.Files = append(got.Files, entry.Name())
		}
	}

	expected := Expectation{Files: []string{".bin", "keepdir", "query.tfquery.hcl"}}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("prepareWorkdir() mismatch (-want +got):\n%s", diff)
	}
}

func TestPrepareWorkdirRefusesTerraformState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name      string
		StateFile string
	}{
		{Name: "current_state", StateFile: "terraform.tfstate"},
		{Name: "backup_state", StateFile: "terraform.tfstate.backup"},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, tc.StateFile), []byte("state"), 0644); err != nil {
				t.Fatalf("write state fixture: %v", err)
			}
			generatedPath := filepath.Join(dir, "projects.tf")
			if err := os.WriteFile(generatedPath, []byte("resource"), 0644); err != nil {
				t.Fatalf("write generated fixture: %v", err)
			}

			type Expectation struct {
				Err              string
				GeneratedRemains bool
				BinExists        bool
			}
			var got Expectation
			if err := prepareWorkdir(dir); err != nil {
				got.Err = err.Error()
			}
			_, err := os.Stat(generatedPath)
			got.GeneratedRemains = err == nil
			_, err = os.Stat(filepath.Join(dir, ".bin"))
			got.BinExists = err == nil

			expected := Expectation{
				Err:              "refusing to clean " + dir + " because it contains Terraform state; use a disposable staging directory",
				GeneratedRemains: true,
			}
			if diff := cmp.Diff(expected, got); diff != "" {
				t.Errorf("prepareWorkdir() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestWriteTerraformScaffoldRequiresQueryCompatibleTerraform(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	type Expectation struct {
		Versions string
		Provider string
		Err      string
	}
	var got Expectation
	if err := writeTerraformScaffold(dir); err != nil {
		got.Err = err.Error()
	} else if versions, err := os.ReadFile(filepath.Join(dir, "versions.tf")); err != nil {
		got.Err = err.Error()
	} else if provider, err := os.ReadFile(filepath.Join(dir, "provider.tf")); err != nil {
		got.Err = err.Error()
	} else {
		got.Versions = string(versions)
		got.Provider = string(provider)
	}

	expected := Expectation{
		Versions: `terraform {
  required_version = ">= 1.14.0"
  required_providers {
    ona = {
      source = "registry.terraform.io/gitpod-io/ona"
    }
  }
}
`,
		Provider: "provider \"ona\" {}\n",
	}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("writeTerraformScaffold() mismatch (-want +got):\n%s", diff)
	}
}
