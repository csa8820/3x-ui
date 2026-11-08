package database

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// Sites saved with the retired English templates would serve 404 after the
// upgrade, so startup moves them onto the closest Chinese template.
func TestInitDBMovesRetiredSiteTemplatesToChineseTemplates(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "x-ui.db")
	if err := InitDB(dbPath); err != nil {
		t.Fatal(err)
	}
	site := model.DecoySite{Name: "old", Config: `{"domain":"a.example.com","template":"studio","title":"Kept"}`}
	if err := db.Create(&site).Error; err != nil {
		t.Fatal(err)
	}
	inbound := model.Inbound{
		Tag: "legacy-site", Port: 443, Protocol: model.VLESS, Settings: `{"clients":[]}`,
		StreamSettings: `{"network":"xhttp","security":"tls","website":{"enabled":true,"template":"sb-agency","domain":"a.example.com"}}`,
	}
	if err := db.Create(&inbound).Error; err != nil {
		t.Fatal(err)
	}
	if err := CloseDB(); err != nil {
		t.Fatal(err)
	}
	if err := InitDB(dbPath); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = CloseDB() }()

	var savedSite model.DecoySite
	if err := db.First(&savedSite, site.Id).Error; err != nil {
		t.Fatal(err)
	}
	var siteConfig map[string]any
	_ = json.Unmarshal([]byte(savedSite.Config), &siteConfig)
	if siteConfig["template"] != "cloud" || siteConfig["title"] != "Kept" || siteConfig["domain"] != "a.example.com" {
		t.Fatalf("site config = %s", savedSite.Config)
	}
	var savedInbound model.Inbound
	if err := db.First(&savedInbound, inbound.Id).Error; err != nil {
		t.Fatal(err)
	}
	var stream struct {
		Network string         `json:"network"`
		Website map[string]any `json:"website"`
	}
	_ = json.Unmarshal([]byte(savedInbound.StreamSettings), &stream)
	if stream.Website["template"] != "group" || stream.Network != "xhttp" || stream.Website["enabled"] != true {
		t.Fatalf("inbound stream = %s", savedInbound.StreamSettings)
	}
}
