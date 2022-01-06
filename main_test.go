package main

import (
	"os"
	"testing"

	"github.com/jetstack/cert-manager/test/acme/dns"
)

var zone = os.Getenv("TEST_ZONE_NAME")

func TestRunSuite(t *testing.T) {
	if zone == "" {
		zone = "dmetest-1641296760802099000.io."
	}
	fixture := dns.NewFixture(&dnsmadeasyDNSProviderSolver{},
		dns.SetResolvedZone(zone),
		dns.SetAllowAmbientCredentials(false),
		dns.SetManifestPath("testdata/dnsmadeasy"),
	)

	fixture.RunConformance(t)
}
