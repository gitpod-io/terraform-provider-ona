package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestWatchPathsSet(t *testing.T) {
	t.Parallel()

	tests := []struct {
		Name     string
		Input    string
		Existing watchPaths
		Expected watchPaths
		Err      string
	}{
		{
			Name:     "adds_path",
			Input:    "docs",
			Existing: watchPaths{"templates"},
			Expected: watchPaths{"templates", "docs"},
		},
		{
			Name:  "rejects_empty_path",
			Err:   "watch path must not be empty",
			Input: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			got := tc.Existing
			err := (&got).Set(tc.Input)
			gotErr := ""
			if err != nil {
				gotErr = err.Error()
			}
			if diff := cmp.Diff(struct {
				Paths watchPaths
				Err   string
			}{Paths: tc.Expected, Err: tc.Err}, struct {
				Paths watchPaths
				Err   string
			}{Paths: got, Err: gotErr}); diff != "" {
				t.Errorf("watchPaths.Set() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestResolveWatchPaths(t *testing.T) {
	t.Parallel()

	providerDir := t.TempDir()
	docsDir := filepath.Join(providerDir, "docs")
	if err := os.Mkdir(docsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	filePath := filepath.Join(docsDir, "index.md")
	if err := os.WriteFile(filePath, []byte("# Docs\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		Name     string
		Paths    []string
		Expected []string
		Err      string
	}{
		{
			Name:     "relative_directory_and_file_are_deduplicated",
			Paths:    []string{"docs", "docs/index.md"},
			Expected: []string{docsDir},
		},
		{
			Name:  "missing_path",
			Paths: []string{"missing"},
			Err:   "watch path " + filepath.Join(providerDir, "missing") + ": stat " + filepath.Join(providerDir, "missing") + ": no such file or directory",
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()

			got, err := resolveWatchPaths(providerDir, tc.Paths)
			gotErr := ""
			if err != nil {
				gotErr = err.Error()
			}
			if diff := cmp.Diff(struct {
				Paths []string
				Err   string
			}{Paths: tc.Expected, Err: tc.Err}, struct {
				Paths []string
				Err   string
			}{Paths: got, Err: gotErr}); diff != "" {
				t.Errorf("resolveWatchPaths() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
