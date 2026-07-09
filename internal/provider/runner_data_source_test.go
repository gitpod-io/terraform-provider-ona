// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"testing"

	v1 "github.com/gitpod-io/terraform-provider-ona/internal/api/go/v1"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccRunnerDataSource(t *testing.T) {
	t.Parallel()

	server := newRunnerAPIServer(t, map[string]*v1.Runner{
		"runner-1": newTestRunnerForDataSource("runner-1", "Frankfurt Runner"),
	})
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRunnerDataSourceConfig(server.URL, "runner-1"),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.ona_runner.test", "id", "runner-1"),
					resource.TestCheckResourceAttr("data.ona_runner.test", "runner_id", "runner-1"),
					resource.TestCheckResourceAttr("data.ona_runner.test", "name", "Frankfurt Runner"),
					resource.TestCheckResourceAttr("data.ona_runner.test", "runner_provider", "aws_ec2"),
					resource.TestCheckResourceAttr("data.ona_runner.test", "kind", "remote"),
					resource.TestCheckResourceAttr("data.ona_runner.test", "cloudformation_template_url", "https://gitpod-flex-releases.s3.amazonaws.com/ec2/stable/gitpod-ec2-runner.json"),
					resource.TestCheckResourceAttr("data.ona_runner.test", "configuration.region", "eu-central-1"),
					resource.TestCheckResourceAttr("data.ona_runner.test", "configuration.release_channel", "stable"),
					resource.TestCheckResourceAttr("data.ona_runner.test", "configuration.auto_update", "true"),
					resource.TestCheckResourceAttr("data.ona_runner.test", "configuration.devcontainer_image_cache_enabled", "true"),
					resource.TestCheckResourceAttr("data.ona_runner.test", "configuration.log_level", "debug"),
					resource.TestCheckResourceAttr("data.ona_runner.test", "configuration.update_window.start", "02:00"),
					resource.TestCheckResourceAttr("data.ona_runner.test", "configuration.update_window.end", "04:00"),
					resource.TestCheckResourceAttr("data.ona_runner.test", "status.phase", "active"),
					resource.TestCheckResourceAttr("data.ona_runner.test", "status.region", "eu-central-1"),
					resource.TestCheckResourceAttr("data.ona_runner.test", "status.message", "ready"),
					resource.TestCheckResourceAttr("data.ona_runner.test", "status.version", "1.2.3"),
					resource.TestCheckResourceAttr("data.ona_runner.test", "status.log_url", "https://example.com/logs"),
					resource.TestCheckResourceAttr("data.ona_runner.test", "status.system_details", "linux/amd64"),
					resource.TestCheckResourceAttr("data.ona_runner.test", "status.support_bundle_url", "https://example.com/support-bundle"),
					resource.TestCheckResourceAttr("data.ona_runner.test", "creator.id", "creator-1"),
					resource.TestCheckResourceAttr("data.ona_runner.test", "creator.principal", "user"),
					resource.TestCheckNoResourceAttr("data.ona_runner.test", "runner_manager_id"),
				),
			},
		},
	})
}

func TestAccRunnersDataSource(t *testing.T) {
	t.Parallel()

	server := newRunnerAPIServer(t, map[string]*v1.Runner{
		"runner-2": newTestRunnerForDataSource("runner-2", "Zurich Runner"),
		"runner-1": newTestRunnerForDataSource("runner-1", "Frankfurt Runner"),
	})
	t.Cleanup(server.Close)

	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { testAccPreCheck(t) },
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: testAccRunnersDataSourceConfig(server.URL),
				Check: resource.ComposeAggregateTestCheckFunc(
					resource.TestCheckResourceAttr("data.ona_runners.test", "id", "runners"),
					resource.TestCheckResourceAttr("data.ona_runners.test", "runners.#", "2"),
					resource.TestCheckResourceAttr("data.ona_runners.test", "runners.0.runner_id", "runner-1"),
					resource.TestCheckResourceAttr("data.ona_runners.test", "runners.0.name", "Frankfurt Runner"),
					resource.TestCheckResourceAttr("data.ona_runners.test", "runners.0.cloudformation_template_url", "https://gitpod-flex-releases.s3.amazonaws.com/ec2/stable/gitpod-ec2-runner.json"),
					resource.TestCheckResourceAttr("data.ona_runners.test", "runners.0.configuration.region", "eu-central-1"),
					resource.TestCheckResourceAttr("data.ona_runners.test", "runners.0.status.phase", "active"),
					resource.TestCheckResourceAttr("data.ona_runners.test", "runners.1.runner_id", "runner-2"),
					resource.TestCheckResourceAttr("data.ona_runners.test", "runners.1.name", "Zurich Runner"),
					resource.TestCheckNoResourceAttr("data.ona_runners.test", "runners.0.runner_manager_id"),
					resource.TestCheckNoResourceAttr("data.ona_runners.test", "runners.1.runner_manager_id"),
				),
			},
		},
	})
}

func testAccRunnerDataSourceConfig(host string, runnerID string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

data "ona_runner" "test" {
  runner_id = %[2]q
}
`, host, runnerID)
}

func testAccRunnersDataSourceConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}

data "ona_runners" "test" {}
`, host)
}

func newTestRunnerForDataSource(id string, name string) *v1.Runner {
	startHour := uint32(2)
	endHour := uint32(4)

	runner := newTestRunner(id, name)
	runner.RunnerManagerId = "runner-manager-1"
	runner.Spec.Configuration = &v1.RunnerConfiguration{
		Region:                        "eu-central-1",
		ReleaseChannel:                v1.RunnerReleaseChannel_RUNNER_RELEASE_CHANNEL_STABLE,
		AutoUpdate:                    true,
		DevcontainerImageCacheEnabled: true,
		LogLevel:                      v1.LogLevel_LOG_LEVEL_DEBUG,
		UpdateWindow: &v1.UpdateWindow{
			StartHour: &startHour,
			EndHour:   &endHour,
		},
	}
	runner.Status.Region = "eu-central-1"
	runner.Status.Message = "ready"
	runner.Status.LogUrl = "https://example.com/logs"
	runner.Status.SystemDetails = "linux/amd64"
	runner.Status.SupportBundleUrl = "https://example.com/support-bundle"
	return runner
}
