// Copyright Ona 2026
// SPDX-License-Identifier: MPL-2.0

package main

import (
	"context"
	"flag"
	"log"

	"github.com/gitpod-io/terraform-provider-ona/internal/provider"
	providerversion "github.com/gitpod-io/terraform-provider-ona/version"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

func main() {
	var debug bool

	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/gitpod-io/ona",
		Debug:   debug,
	}

	err := providerserver.Serve(context.Background(), provider.New(providerversion.ProviderVersion), opts)

	if err != nil {
		log.Fatal(err.Error())
	}
}
