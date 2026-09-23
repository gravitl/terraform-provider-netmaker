// Terraform provider for managing Netmaker networks, enrollment keys,
// devices, nodes, and ext clients.
package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/gravitl/terraform-provider-netmaker/internal/provider"
)

// Run `go generate ./...` to regenerate docs/ from the resource/data
// source schemas (their Description fields) after changing one.
// --provider-name/--rendered-provider-name are needed because tfplugindocs
// otherwise infers the provider name from this directory's name, which
// only works if the directory (and the GitHub repo it's cloned from) is
// literally named terraform-provider-netmaker.
//go:generate go tool tfplugindocs generate --provider-name netmaker --rendered-provider-name netmaker

// version is set via -ldflags at build time by goreleaser.
var version string = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run the provider with support for debuggers like delve")
	flag.Parse()

	opts := providerserver.ServeOpts{
		Address: "registry.terraform.io/gravitl/netmaker",
		Debug:   debug,
	}

	if err := providerserver.Serve(context.Background(), provider.New(version), opts); err != nil {
		log.Fatal(err.Error())
	}
}
