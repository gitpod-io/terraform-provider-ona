// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	managementclient "github.com/gitpod-io/terraform-provider-ona/internal/managementclient"
)

const defaultHost = "https://app.gitpod.io"

var errMissingToken = errors.New("missing Ona token: set provider token or ONA_TOKEN")

func newManagementPlane(host, token, userAgent string) (*managementclient.ManagementPlane, string, error) {
	if strings.TrimSpace(host) == "" {
		host = os.Getenv("ONA_HOST")
	}
	if strings.TrimSpace(host) == "" {
		host = defaultHost
	}
	host = strings.TrimSpace(host)
	if strings.TrimSpace(token) == "" {
		token = os.Getenv("ONA_TOKEN")
	}
	if strings.TrimSpace(token) == "" {
		return nil, "", errMissingToken
	}
	if !strings.Contains(host, "://") {
		host = "https://" + host
	}
	parsed, err := url.Parse(host)
	if err != nil {
		return nil, "", fmt.Errorf("parse Ona host: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.Host == "" {
		return nil, "", fmt.Errorf("invalid Ona host %q", host)
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/api"
	parsed.RawQuery = ""
	parsed.Fragment = ""
	baseURL := parsed.String()
	client, err := managementclient.New(baseURL, managementclient.WithAccessToken(token), managementclient.WithUserAgent(userAgent))
	if err != nil {
		return nil, "", fmt.Errorf("create Ona management client: %w", err)
	}
	return client, baseURL, nil
}
