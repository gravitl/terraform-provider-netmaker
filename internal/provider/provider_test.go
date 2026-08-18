package provider

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// testProtoV6ProviderFactories runs the provider in-process (via
// reattach) — no registry, no separate binary build.
var testProtoV6ProviderFactories = map[string]func() (tfprotov6.ProviderServer, error){
	"netmaker": providerserver.NewProtocol6WithError(New("test")()),
}

// testPreCheck validates that a Netmaker server and credentials are
// configured before any test runs. These tests hit a real server (no
// docker-compose harness / mocking) — set NETMAKER_API_URL and
// NETMAKER_API_TOKEN (NETMAKER_TENANT_ID if needed) to a real or
// disposable test Netmaker instance before running `go test` with
// TF_ACC=1.
func testPreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("NETMAKER_API_URL") == "" {
		t.Fatal("NETMAKER_API_URL must be set to run these tests, pointing at a real Netmaker server")
	}
	if os.Getenv("NETMAKER_API_TOKEN") == "" {
		t.Fatal("NETMAKER_API_TOKEN must be set to run these tests")
	}
}
