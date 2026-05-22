package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/lucasaarch/terraform-provider-dokploy/internal/provider"
)

// version is set at build time by GoReleaser via -ldflags.
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "set to true to run the provider with debugger support")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/lucasaarch/dokploy",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err.Error())
	}
}
