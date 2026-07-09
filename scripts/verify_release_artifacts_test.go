package main

import (
	"archive/zip"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestVerifyReleaseArtifacts(t *testing.T) {
	t.Parallel()

	const version = "0.1.0-beta.1"

	type Expectation struct {
		Success  bool
		Contains []bool
	}

	tests := []struct {
		Name     string
		Mutate   func(t *testing.T, dir string)
		Args     []string
		Substrs  []string
		Expected Expectation
	}{
		{
			Name:    "valid_unsigned_release",
			Substrs: []string{"Verified Terraform provider release artifacts for 0.1.0-beta.1"},
			Expected: Expectation{
				Success:  true,
				Contains: []bool{true},
			},
		},
		{
			Name: "extra_artifact",
			Mutate: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, dir, "unexpected.txt", "extra")
			},
			Substrs: []string{"release artifact inventory mismatch"},
			Expected: Expectation{
				Contains: []bool{true},
			},
		},
		{
			Name: "missing_artifact",
			Mutate: func(t *testing.T, dir string) {
				t.Helper()
				removeFile(t, dir, "terraform-provider-ona_0.1.0-beta.1_linux_arm64.zip")
			},
			Substrs: []string{"release artifact inventory mismatch"},
			Expected: Expectation{
				Contains: []bool{true},
			},
		},
		{
			Name: "missing_checksum_entry",
			Mutate: func(t *testing.T, dir string) {
				t.Helper()
				writeChecksums(t, dir, []string{
					"terraform-provider-ona_0.1.0-beta.1_linux_amd64.zip",
					"terraform-provider-ona_0.1.0-beta.1_linux_arm64.zip",
				})
			},
			Substrs: []string{"checksum file does not match expected release artifacts"},
			Expected: Expectation{
				Contains: []bool{true},
			},
		},
		{
			Name: "unexpected_zip_contents",
			Mutate: func(t *testing.T, dir string) {
				t.Helper()
				writeZip(t, dir, "terraform-provider-ona_0.1.0-beta.1_linux_amd64.zip", map[string]string{
					"terraform-provider-ona_v0.1.0-beta.1": "binary",
					"README.txt":                           "unexpected",
				})
				writeChecksums(t, dir, releaseChecksumFiles(version))
			},
			Substrs: []string{"unexpected zip contents"},
			Expected: Expectation{
				Contains: []bool{true},
			},
		},
		{
			Name: "invalid_manifest_metadata",
			Mutate: func(t *testing.T, dir string) {
				t.Helper()
				writeFile(t, dir, "terraform-provider-ona_0.1.0-beta.1_manifest.json", `{"version":1,"metadata":{"protocol_versions":["5.0"]}}`)
				writeChecksums(t, dir, releaseChecksumFiles(version))
			},
			Substrs: []string{"manifest must declare version 1 and Terraform protocol version 6.0"},
			Expected: Expectation{
				Contains: []bool{true},
			},
		},
		{
			Name:    "required_signature_missing",
			Args:    []string{"--require-signature"},
			Substrs: []string{"release artifact inventory mismatch"},
			Expected: Expectation{
				Contains: []bool{true},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeValidReleaseFixture(t, dir, version)
			if tc.Mutate != nil {
				tc.Mutate(t, dir)
			}

			args := append([]string{verifierScriptPath(t), dir, version}, tc.Args...)
			cmd := exec.CommandContext(t.Context(), "bash", args...)
			output, err := cmd.CombinedOutput()

			got := Expectation{Success: err == nil}
			for _, substr := range tc.Substrs {
				got.Contains = append(got.Contains, strings.Contains(string(output), substr))
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("verify-release-artifacts.sh output:\n%s\nmismatch (-want +got):\n%s", string(output), diff)
			}
		})
	}
}

func verifierScriptPath(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	return filepath.Join(wd, "verify-release-artifacts.sh")
}

func writeValidReleaseFixture(t *testing.T, dir string, version string) {
	t.Helper()

	binaryName := fmt.Sprintf("terraform-provider-ona_v%s", version)
	writeZip(t, dir, fmt.Sprintf("terraform-provider-ona_%s_linux_amd64.zip", version), map[string]string{
		binaryName: "amd64 binary",
	})
	writeZip(t, dir, fmt.Sprintf("terraform-provider-ona_%s_linux_arm64.zip", version), map[string]string{
		binaryName: "arm64 binary",
	})
	writeFile(t, dir, fmt.Sprintf("terraform-provider-ona_%s_manifest.json", version), `{"version":1,"metadata":{"protocol_versions":["6.0"]}}`)
	writeChecksums(t, dir, releaseChecksumFiles(version))
}

func releaseChecksumFiles(version string) []string {
	return []string{
		fmt.Sprintf("terraform-provider-ona_%s_linux_amd64.zip", version),
		fmt.Sprintf("terraform-provider-ona_%s_linux_arm64.zip", version),
		fmt.Sprintf("terraform-provider-ona_%s_manifest.json", version),
	}
}

func writeChecksums(t *testing.T, dir string, files []string) {
	t.Helper()

	var builder strings.Builder
	for _, name := range files {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read checksum fixture %s: %v", name, err)
		}
		sum := sha256.Sum256(data)
		fmt.Fprintf(&builder, "%x  %s\n", sum, name)
	}
	writeFile(t, dir, "terraform-provider-ona_0.1.0-beta.1_SHA256SUMS", builder.String())
}

func writeZip(t *testing.T, dir string, name string, files map[string]string) {
	t.Helper()

	path := filepath.Join(dir, name)
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip fixture %s: %v", name, err)
	}

	archive := zip.NewWriter(file)
	for entryName, contents := range files {
		writer, err := archive.Create(entryName)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", entryName, err)
		}
		if _, err := writer.Write([]byte(contents)); err != nil {
			t.Fatalf("write zip entry %s: %v", entryName, err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatalf("close zip fixture %s: %v", name, err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close zip file %s: %v", name, err)
	}
}

func writeFile(t *testing.T, dir string, name string, contents string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, name), []byte(contents), 0644); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
}

func removeFile(t *testing.T, dir string, name string) {
	t.Helper()

	if err := os.Remove(filepath.Join(dir, name)); err != nil {
		t.Fatalf("remove fixture %s: %v", name, err)
	}
}
