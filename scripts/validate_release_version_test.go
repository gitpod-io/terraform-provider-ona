package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestValidateReleaseVersion(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Success      bool
		Contains     string
		GitHubOutput string
	}

	tests := []struct {
		Name          string
		StableVersion string
		BetaLine      string
		Heading       string
		Tags          []string
		Args          []string
		GitHubOutput  bool
		Expected      Expectation
	}{
		{
			Name:          "stable_valid_without_tags",
			StableVersion: "0.2.0",
			BetaLine:      "0.3.0-beta",
			Heading:       "## 0.2.0 (Unreleased)",
			Args:          []string{"--channel", "stable"},
			Expected: Expectation{
				Success: true,
			},
		},
		{
			Name:          "stable_rejects_prerelease_manifest",
			StableVersion: "0.2.0-beta.1",
			BetaLine:      "0.3.0-beta",
			Heading:       "## 0.2.0-beta.1 (Unreleased)",
			Args:          []string{"--channel", "stable"},
			Expected: Expectation{
				Contains: "must contain a stable SemVer without prerelease metadata",
			},
		},
		{
			Name:          "stable_changelog_match_is_literal",
			StableVersion: "0.2.0",
			BetaLine:      "0.3.0-beta",
			Heading:       "## 0x2x0 (Unreleased)",
			Args:          []string{"--channel", "stable"},
			Expected: Expectation{
				Contains: "first version heading must match",
			},
		},
		{
			Name:          "stable_release_beats_matching_prerelease",
			StableVersion: "0.2.0",
			BetaLine:      "0.3.0-beta",
			Heading:       "## 0.2.0 (Unreleased)",
			Tags:          []string{"v0.2.0-beta.2"},
			Args:          []string{"--channel", "stable"},
			Expected: Expectation{
				Success: true,
			},
		},
		{
			Name:          "stable_lower_minor_rejected_after_higher_minor_prerelease",
			StableVersion: "0.2.0",
			BetaLine:      "0.3.0-beta",
			Heading:       "## 0.2.0 (Unreleased)",
			Tags:          []string{"v0.3.0-beta.1"},
			Args:          []string{"--channel", "stable"},
			Expected: Expectation{
				Contains: "must be greater than highest existing tag v0.3.0-beta.1",
			},
		},
		{
			Name:          "stable_no_tag_precedence_skips_existing_higher_tag",
			StableVersion: "0.2.0",
			BetaLine:      "0.3.0-beta",
			Heading:       "## 0.2.0 (Unreleased)",
			Tags:          []string{"v0.3.0-beta.1"},
			Args:          []string{"--channel", "stable", "--no-tag-precedence"},
			Expected: Expectation{
				Success: true,
			},
		},
		{
			Name:          "stable_expected_tag_matches_version",
			StableVersion: "0.2.0",
			BetaLine:      "0.3.0-beta",
			Heading:       "## 0.2.0 (Unreleased)",
			Args:          []string{"--channel", "stable", "--expect-tag", "v0.2.0", "--no-tag-precedence"},
			Expected: Expectation{
				Success: true,
			},
		},
		{
			Name:          "stable_expected_tag_mismatch_rejected",
			StableVersion: "0.2.0",
			BetaLine:      "0.3.0-beta",
			Heading:       "## 0.2.0 (Unreleased)",
			Args:          []string{"--channel", "stable", "--expect-tag", "v0.2.1", "--no-tag-precedence"},
			Expected: Expectation{
				Contains: "must match expected version 0.2.1",
			},
		},
		{
			Name:          "beta_resolves_first_number_without_tags",
			StableVersion: "0.2.0",
			BetaLine:      "0.3.0-beta",
			Heading:       "## 0.2.0 (Unreleased)",
			Args:          []string{"--channel", "beta", "--github-output"},
			GitHubOutput:  true,
			Expected: Expectation{
				Success:      true,
				GitHubOutput: "version=v0.3.0-beta.1\nversion_no_v=0.3.0-beta.1\n",
			},
		},
		{
			Name:          "beta_increments_latest_matching_tag",
			StableVersion: "0.2.0",
			BetaLine:      "0.3.0-beta",
			Heading:       "## 0.2.0 (Unreleased)",
			Tags:          []string{"v0.3.0-beta.1", "v0.3.0-beta.9", "v0.2.0-beta.99"},
			Args:          []string{"--channel", "beta", "--github-output"},
			GitHubOutput:  true,
			Expected: Expectation{
				Success:      true,
				GitHubOutput: "version=v0.3.0-beta.10\nversion_no_v=0.3.0-beta.10\n",
			},
		},
		{
			Name:          "beta_expected_tag_matches_computed_next_version",
			StableVersion: "0.2.0",
			BetaLine:      "0.3.0-beta",
			Heading:       "## 0.2.0 (Unreleased)",
			Tags:          []string{"v0.3.0-beta.1"},
			Args:          []string{"--channel", "beta", "--expect-tag", "v0.3.0-beta.2"},
			Expected: Expectation{
				Success: true,
			},
		},
		{
			Name:          "beta_expected_tag_mismatch_rejected",
			StableVersion: "0.2.0",
			BetaLine:      "0.3.0-beta",
			Heading:       "## 0.2.0 (Unreleased)",
			Tags:          []string{"v0.3.0-beta.1"},
			Args:          []string{"--channel", "beta", "--expect-tag", "v0.3.0-beta.3"},
			Expected: Expectation{
				Contains: "must match expected version 0.3.0-beta.3",
			},
		},
		{
			Name:          "beta_rejects_numbered_manifest_version",
			StableVersion: "0.2.0",
			BetaLine:      "0.3.0-beta.1",
			Heading:       "## 0.2.0 (Unreleased)",
			Args:          []string{"--channel", "beta"},
			Expected: Expectation{
				Contains: "must contain a beta line such as 0.3.0-beta",
			},
		},
		{
			Name:          "beta_rejected_after_matching_stable_tag",
			StableVersion: "0.3.0",
			BetaLine:      "0.3.0-beta",
			Heading:       "## 0.3.0 (Unreleased)",
			Tags:          []string{"v0.3.0"},
			Args:          []string{"--channel", "beta"},
			Expected: Expectation{
				Contains: "must be greater than highest existing tag v0.3.0",
			},
		},
		{
			Name:          "beta_no_tag_precedence_skips_matching_stable_tag",
			StableVersion: "0.3.0",
			BetaLine:      "0.3.0-beta",
			Heading:       "## 0.3.0 (Unreleased)",
			Tags:          []string{"v0.3.0"},
			Args:          []string{"--channel", "beta", "--no-tag-precedence"},
			Expected: Expectation{
				Success: true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			writeReleaseVersionFixture(t, dir, tc.StableVersion, tc.BetaLine, tc.Heading)
			if len(tc.Tags) > 0 {
				createGitTags(t, dir, tc.Tags)
			}

			args := append([]string{validateReleaseVersionScriptPath(t)}, tc.Args...)
			cmd := exec.CommandContext(t.Context(), "bash", args...)
			cmd.Dir = dir

			var githubOutput string
			if tc.GitHubOutput {
				githubOutput = filepath.Join(dir, "github-output")
				cmd.Env = append(os.Environ(), "GITHUB_OUTPUT="+githubOutput)
			}

			output, err := cmd.CombinedOutput()

			got := Expectation{
				Success: err == nil,
			}
			if tc.Expected.Contains != "" && strings.Contains(string(output), tc.Expected.Contains) {
				got.Contains = tc.Expected.Contains
			}
			if tc.GitHubOutput {
				data, readErr := os.ReadFile(githubOutput)
				if readErr != nil {
					t.Fatalf("read GITHUB_OUTPUT fixture: %v", readErr)
				}
				got.GitHubOutput = string(data)
			}

			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("validate-release-version.sh mismatch (-want +got):\n%s\noutput:\n%s", diff, string(output))
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

func writeReleaseVersionFixture(t *testing.T, dir string, stableVersion string, betaLine string, heading string) {
	t.Helper()

	versionDir := filepath.Join(dir, "version")
	if err := os.MkdirAll(versionDir, 0755); err != nil {
		t.Fatalf("create version fixture directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "STABLE_VERSION"), []byte(stableVersion+"\n"), 0644); err != nil {
		t.Fatalf("write version/STABLE_VERSION fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(versionDir, "BETA_VERSION"), []byte(betaLine+"\n"), 0644); err != nil {
		t.Fatalf("write version/BETA_VERSION fixture: %v", err)
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
	runGit(t, dir, "add", "version/STABLE_VERSION", "version/BETA_VERSION", "CHANGELOG.md")
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
