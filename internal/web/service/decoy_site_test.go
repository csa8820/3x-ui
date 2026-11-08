package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/website"
)

func TestSiteAssociationChoosesSupportedMode(t *testing.T) {
	for _, tc := range []struct {
		name, protocol, network, security, want string
		port                                    int
	}{
		{"xhttp", "vless", "xhttp", "tls", "path", 443},
		{"websocket", "vmess", "ws", "tls", "path", 443},
		{"grpc", "vless", "grpc", "tls", "path", 443},
		{"httpupgrade", "trojan", "httpupgrade", "tls", "path", 443},
		{"raw-vless", "vless", "tcp", "tls", "fallback", 443},
		{"raw-trojan", "trojan", "tcp", "tls", "fallback", 443},
		{"reality", "vless", "tcp", "reality", "reality", 443},
		{"shadowsocks", "shadowsocks", "tcp", "none", "independent", 8388},
		{"hysteria", "hysteria", "hysteria", "tls", "independent", 443},
		{"tuic", "tuic", "", "", "independent", 443},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, _ := json.Marshal(map[string]any{"network": tc.network, "security": tc.security, "website": map[string]any{"enabled": true, "siteId": 7, "mode": "auto"}, "xhttpSettings": map[string]any{"path": "/connect/"}})
			ib := &model.Inbound{Protocol: model.Protocol(tc.protocol), Port: tc.port, StreamSettings: string(raw)}
			if err := applySiteProfile(ib, website.Config{SiteID: 7, Enabled: true, Domain: "site.example.com", Port: 443, Template: "news"}); err != nil {
				t.Fatal(err)
			}
			stream, _ := website.Parse(ib.StreamSettings)
			if stream.Website.Mode != tc.want || stream.Website.Template != "news" {
				t.Fatalf("wrong association: %+v", stream.Website)
			}
			if tc.want == "independent" && strings.Contains(ib.StreamSettings, `"security":"none"`) == false && tc.security == "none" {
				t.Fatal("proxy security was changed")
			}
		})
	}
}

func TestSiteProfileCRUDAndReferenceProtection(t *testing.T) {
	previous := isNeedXrayRestart.Load()
	defer isNeedXrayRestart.Store(previous)
	previousReconcile := isNeedXrayReconcile.Load()
	defer isNeedXrayReconcile.Store(previousReconcile)
	setupSettingTestDB(t)
	svc := &DecoySiteService{}
	row := &model.DecoySite{Name: "News", Config: `{"domain":"site.example.com","autoTLS":true,"template":"news","port":443}`}
	if err := svc.Save(row); err != nil {
		t.Fatal(err)
	}
	ib := &model.Inbound{Tag: "associated-website", Port: 443, Protocol: model.VLESS, Enable: true, Settings: `{"clients":[],"decryption":"none"}`, StreamSettings: `{"network":"xhttp","security":"tls","xhttpSettings":{"path":"/connect/"},"tlsSettings":{"certificates":[]}}`}
	var stream map[string]any
	_ = json.Unmarshal([]byte(ib.StreamSettings), &stream)
	stream["website"] = map[string]any{"enabled": true, "siteId": row.Id, "mode": "auto"}
	data, _ := json.Marshal(stream)
	ib.StreamSettings = string(data)
	if err := resolveInboundSite(ib); err != nil {
		t.Fatal(err)
	}
	if err := database.GetDB().Create(ib).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(row.Id); err == nil {
		t.Fatal("deleted a referenced website")
	}
	row.Config = `{"domain":"site.example.com","autoTLS":true,"template":"brand","title":"Updated site","port":443}`
	if err := svc.Save(row); err != nil {
		t.Fatal(err)
	}
	var saved model.Inbound
	if err := database.GetDB().First(&saved, ib.Id).Error; err != nil {
		t.Fatal(err)
	}
	linked, _ := website.Parse(saved.StreamSettings)
	if linked.Website.Template != "brand" || linked.Website.Title != "Updated site" || linked.Website.Mode != "path" {
		t.Fatalf("profile did not synchronize: %+v", linked.Website)
	}
	if err := database.GetDB().Delete(ib).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.Delete(row.Id); err != nil {
		t.Fatal(err)
	}
}

func TestSiteAssociationRejectsOpaqueTCPPortCollision(t *testing.T) {
	ib := &model.Inbound{Protocol: model.Shadowsocks, Port: 443, StreamSettings: `{"network":"tcp","security":"none","website":{"enabled":true,"mode":"auto"}}`}
	err := applySiteProfile(ib, website.Config{Enabled: true, Domain: "site.example.com", Port: 443})
	if err == nil || !strings.Contains(err.Error(), "cannot share the same TCP port") {
		t.Fatalf("unexpected collision result: %v", err)
	}
}

// Linking a site that has its own auto certificate used to replace the inbound's
// certificate, so clients using the inbound's real domain failed the ACME whitelist.
func TestSiteAssociationKeepsInboundCertificate(t *testing.T) {
	certFile, keyFile := writeTestCertificate(t)
	for _, tc := range []struct{ name, protocol, network, want string }{
		{"path", "vless", "xhttp", "path"},
		{"fallback", "trojan", "tcp", "fallback"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tlsSettings := map[string]any{"serverName": "golden-fixture.test", "certificates": []any{map[string]any{"certificateFile": certFile, "keyFile": keyFile}}}
			raw, _ := json.Marshal(map[string]any{"network": tc.network, "security": "tls", "tlsSettings": tlsSettings, "xhttpSettings": map[string]any{"path": "/connect/"}, "website": map[string]any{"enabled": true, "siteId": 7, "mode": "auto"}})
			ib := &model.Inbound{Tag: "own-cert-" + tc.name, Protocol: model.Protocol(tc.protocol), Port: 443, Settings: `{"clients":[]}`, StreamSettings: string(raw), ShareAddrStrategy: "custom", ShareAddr: "golden-fixture.test"}
			if err := applySiteProfile(ib, website.Config{SiteID: 7, Enabled: true, Domain: "site.example.com", Port: 443, Template: "news", AutoTLS: true, Email: "a@example.com"}); err != nil {
				t.Fatal(err)
			}
			var got struct {
				TLS struct {
					ServerName   string           `json:"serverName"`
					Certificates []map[string]any `json:"certificates"`
				} `json:"tlsSettings"`
			}
			_ = json.Unmarshal([]byte(ib.StreamSettings), &got)
			if got.TLS.ServerName != "golden-fixture.test" || len(got.TLS.Certificates) != 1 || got.TLS.Certificates[0]["certificateFile"] != certFile {
				t.Fatalf("inbound TLS was overwritten: %+v", got.TLS)
			}
			if ib.ShareAddr != "golden-fixture.test" {
				t.Fatalf("share address was overwritten: %q", ib.ShareAddr)
			}
			stream, _ := website.Parse(ib.StreamSettings)
			if stream.Website.Mode != tc.want || stream.Website.AutoTLS || stream.Website.CertificateFile != "" || stream.Website.Domain != "golden-fixture.test" {
				t.Fatalf("website does not reuse the inbound certificate: %+v", stream.Website)
			}
			if err := validateInboundWebsite(ib); err != nil {
				t.Fatalf("linked inbound rejected: %v", err)
			}
		})
	}
}
