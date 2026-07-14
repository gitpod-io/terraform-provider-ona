// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package runner

import (
	"testing"

	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
)

func TestImportableRunner(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		runner *v1.Runner
		want   bool
	}{
		{name: "remote_aws", runner: &v1.Runner{Kind: v1.RunnerKind_RUNNER_KIND_REMOTE, Provider: v1.RunnerProvider_RUNNER_PROVIDER_AWS_EC2}, want: true},
		{name: "remote_gcp", runner: &v1.Runner{Kind: v1.RunnerKind_RUNNER_KIND_REMOTE, Provider: v1.RunnerProvider_RUNNER_PROVIDER_GCP}, want: true},
		{name: "managed", runner: &v1.Runner{Kind: v1.RunnerKind_RUNNER_KIND_LOCAL_CONFIGURATION, Provider: v1.RunnerProvider_RUNNER_PROVIDER_AWS_EC2}},
		{name: "unsupported_provider", runner: &v1.Runner{Kind: v1.RunnerKind_RUNNER_KIND_REMOTE, Provider: v1.RunnerProvider_RUNNER_PROVIDER_DEV_AGENT}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := importableRunner(tc.runner); got != tc.want {
				t.Errorf("importableRunner() = %t, want %t", got, tc.want)
			}
		})
	}
}

func TestRunnerProvidersFromNames(t *testing.T) {
	t.Parallel()

	got, err := runnerProvidersFromNames([]string{"gcp", "aws_ec2"})
	if err != nil {
		t.Fatalf("runnerProvidersFromNames() error: %v", err)
	}
	if len(got) != 2 || got[0] != v1.RunnerProvider_RUNNER_PROVIDER_GCP || got[1] != v1.RunnerProvider_RUNNER_PROVIDER_AWS_EC2 {
		t.Fatalf("runnerProvidersFromNames() = %#v", got)
	}
	if _, err := runnerProvidersFromNames([]string{"managed"}); err == nil {
		t.Fatal("runnerProvidersFromNames() accepted an unsupported provider")
	}
}
