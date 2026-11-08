package website

import (
	"crypto/tls"
	"slices"
	"strings"
	"testing"

	"golang.org/x/crypto/acme"
)

func TestWebsiteAutoTLSLimitsIssuanceToConfiguredDomain(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	config, err := TLSConfig(Stream{Website: Config{Enabled: true, Domain: "site.example.com", AutoTLS: true}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(config.NextProtos, "h2") || !slices.Contains(config.NextProtos, acme.ALPNProto) {
		t.Fatalf("missing HTTP/2 or ACME ALPN: %v", config.NextProtos)
	}
	_, err = config.GetCertificate(&tls.ClientHelloInfo{ServerName: "attacker.example.com"})
	if err == nil || !strings.Contains(err.Error(), "not configured in HostWhitelist") {
		t.Fatalf("unexpected issuance authorization result: %v", err)
	}
}
