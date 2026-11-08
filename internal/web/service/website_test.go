package service

import (
	"encoding/json"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/website"
)

func TestWebsiteCreateWithoutManualCertificate(t *testing.T) {
	setupSettingTestDB(t)
	ib := &model.Inbound{Enable: true, Port: 443, Protocol: model.VLESS,
		Settings:       `{"decryption":"none","clients":[{"id":"11111111-2222-4333-8444-555555555555","email":"website-client","enable":true}]}`,
		StreamSettings: `{"network":"xhttp","security":"tls","xhttpSettings":{"path":"/connect-test/"},"tlsSettings":{"serverName":"site.example.com","certificates":[]},"website":{"enabled":true,"domain":"site.example.com","autoTLS":true}}`}
	created, _, err := (&InboundService{}).AddInbound(ib)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := (&InboundService{}).GetInbound(created.Id)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Port != 443 || stored.Listen != "" {
		t.Fatalf("public endpoint changed: %s:%d", stored.Listen, stored.Port)
	}
	cfg, err := (&XrayService{}).GetXrayConfig()
	if err != nil {
		t.Fatal(err)
	}
	for _, core := range cfg.InboundConfigs {
		if core.Tag != created.Tag {
			continue
		}
		var listen string
		if err := json.Unmarshal(core.Listen, &listen); err != nil {
			t.Fatal(err)
		}
		if listen != website.BackendAddress(created.Tag) {
			t.Fatalf("backend socket = %q", listen)
		}
		return
	}
	t.Fatal("website backend was not generated")
}

func TestWebsiteRejectsUnsupportedCombination(t *testing.T) {
	ib := &model.Inbound{Port: 443, Protocol: model.VLESS, StreamSettings: `{"network":"xhttp","security":"tls","xhttpSettings":{"path":"/"},"website":{"enabled":true,"domain":"site.example.com","autoTLS":true}}`}
	if err := validateInboundWebsite(ib); err == nil || err.Error() != "website requires a dedicated XHTTP path such as /connect-a83f/" {
		t.Fatalf("unexpected path validation: %v", err)
	}
}
