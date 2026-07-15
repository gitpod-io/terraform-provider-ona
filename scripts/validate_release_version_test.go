package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateReleaseVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name     string
		Version  string
		Heading  string
		Tags     []string
		Args     []string
		Success  bool
		Contains string
	}{
		{
			Name:    "valid_prerelease_without_tags",
			Version: "0.2.0-beta.1",
			Heading: "## 0.2.0-beta.1 (Unreleased)",
			Success: true,
		},
		{
			Name:     "changelog_match_is_literal",
			Version:  "0.2.0-beta.1",
			Heading:  "## 0x2x0-betax1 (Unreleased)",
			Contains: "first version heading must match",
		},
		{
			Name:    "stable_release_beats_matching_prerelease",
			Version: "0.2.0",
			Heading: "## 0.2.0 (Unreleased)",
			Tags:    []string{"v0.2.0-beta.1"},
			Success: true,
		},
		{
			Name:     "lower_minor_prerelease_rejected_after_higher_minor_prerelease",
			Version:  "0.1.0-beta.28",
			Heading:  "## 0.1.0-beta.28 (Unreleased)",
			Tags:     []string{"v0.2.0-beta.0"},
			Contains: "must be greater than highest existing tag v0.2.0-beta.0",
		},
		{
			Name:     "same_version_as_existing_tag_rejected",
			Version:  "0.2.0-beta.1",
			Heading:  "## 0.2.0-beta.1 (Unreleased)",
			Tags:     []string{"v0.2.0-beta.1"},
			Contains: "must be greater than highest existing tag v0.2.0-beta.1",
		},
		{
			Name:    "no_tag_precedence_skips_existing_higher_tag",
			Version: "0.1.0-beta.28",
			Heading: "## 0.1.0-beta.28 (Unreleased)",
			Tags:    []string{"v0.2.0-beta.0"},
			Args:    []string{"--no-tag-precedence"},
			Success: true,
		},
		{
			Name:    "expected_tag_matches_version",
			Version: "0.2.0-beta.1",
			Heading: "## 0.2.0-beta.1 (Unreleased)",
			Args:    []string{"--expect-tag", "v0.2.0-beta.1", "--no-tag-precedence"},
			Success: true,
		},
		{
			Name:     "expected_tag_mismatch_rejected",
			Version:  "0.2.0-beta.1",
			Heading:  "## 0.2.0-beta.1 (Unreleased)",
			Args:     []string{"--expect-tag", "v0.2.0-beta.2", "--no-tag-precedence"},
			Contains: "must match expected version 0.2.0-beta.2",
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeReleaseVersionFixture(t, dir, tc.Version, tc.Heading)
			if len(tc.Tags) > 0 {
				createGitTags(t, dir, tc.Tags)
			}

			args := append([]string{validateReleaseVersionScriptPath(t)}, tc.Args...)
			cmd := exec.CommandContext(t.Context(), "bash", args...)
			cmd.Dir = dir
			output, err := cmd.CombinedOutput()

			if tc.Success && err != nil {
				t.Fatalf("validate-release-version.sh failed unexpectedly:\n%s", string(output))
			}
			if !tc.Success && err == nil {
				t.Fatalf("validate-release-version.sh succeeded unexpectedly:\n%s", string(output))
			}
			if tc.Contains != "" && !strings.Contains(string(output), tc.Contains) {
				t.Fatalf("validate-release-version.sh output:\n%s\nmissing substring: %s", string(output), tc.Contains)
			}
		})
	}
}

func validateReleaseVersionScriptPath(t *testing.T) string {
	t.Helper()

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	return filepath.Join(wd, "validate-release-version.sh")
}

func writeReleaseVersionFixture(t *testing.T, dir string, version string, heading string) {
	t.Helper()

	versionDir := filepath.Join(dir, "version")
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		t.Fatalf("create version fixture directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "VERSION"), []byte(version+"\n"), 0644); err != nil {
		t.Fatalf("write version/VERSION fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte(heading+"\n"), 0644); err != nil {
		t.Fatalf("write CHANGELOG fixture: %v", err)
	}
}

func createGitTags(t *testing.T, dir string, tags []string) {
	t.Helper()

	runGit(t, dir, "init", "-q")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Release Test")
	runGit(t, dir, "add", "version/VERSION", "CHANGELOG.md")
	runGit(t, dir, "commit", "-q", "-m", "fixture")
	for _, tag := range tags {
		runGit(t, dir, "tag", tag)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
	}
}
