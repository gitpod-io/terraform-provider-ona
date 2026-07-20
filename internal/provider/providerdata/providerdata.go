// Copyright IBM Corp. 2021, 2026
// SPDX-License-Identifier: MPL-2.0

package providerdata

import (
	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
)

type Data struct {
	Client     *managementclient.ManagementPlane
	APIBaseURL string
	UserAgent  string
}
