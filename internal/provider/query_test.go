// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"

	testresource "github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// QueryTestCase creates a hermetic Terraform 1.14+ Query test case for the
// provider package's fake API servers.
func QueryTestCase(host string, step testresource.TestStep) testresource.TestCase {
	return testresource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(tfversion.Version1_14_0),
		},
		PreCheck:                 func() {},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []testresource.TestStep{
			{
				Config: QueryProviderConfig(host),
			},
			step,
		},
	}
}

// QueryProviderConfig returns provider configuration for a fake Ona API.
func QueryProviderConfig(host string) string {
	return fmt.Sprintf(`
provider "ona" {
  host  = %[1]q
  token = "test-token"
}
`, host)
}
