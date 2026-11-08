package model

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/website"
)

func TestWebsiteCoreConfigKeepsPublicEndpointInDatabase(t *testing.T) {
	raw := `{"network":"xhttp","security":"tls","xhttpSettings":{"path":"/connect-test/"},"tlsSettings":{"serverName":"example.com"},"website":{"enabled":true,"domain":"example.com","autoTLS":true}}`
	ib := &Inbound{Tag: "website-test", Port: 443, Protocol: VLESS, StreamSettings: raw, Settings: `{"decryption":"none","clients":[]}`}
	core := ib.GenXrayInboundConfig()
	var listen string
	if err := json.Unmarshal(core.Listen, &listen); err != nil {
		t.Fatal(err)
	}
	if listen != website.BackendAddress(ib.Tag) || core.Port != 0 {
		t.Fatalf("core unexpectedly bound public endpoint: %s:%d", listen, core.Port)
	}
	var stream map[string]any
	if err := json.Unmarshal(core.StreamSettings, &stream); err != nil {
		t.Fatal(err)
	}
	if stream["security"] != "none" || stream["tlsSettings"] != nil || stream["website"] != nil {
		t.Fatalf("website metadata or TLS reached the internal backend: %s", core.StreamSettings)
	}
	if !strings.Contains(string(core.StreamSettings), "/connect-test/") {
		t.Fatal("XHTTP path was lost")
	}
	if ib.Port != 443 || ib.StreamSettings != raw {
		t.Fatal("public config was mutated and would break subscriptions")
	}
}
