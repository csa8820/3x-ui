package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/website"
	"gorm.io/gorm"
)

type DecoySiteService struct{}

func (s *DecoySiteService) List() ([]model.DecoySite, error) {
	var rows []model.DecoySite
	err := database.GetDB().Order("id").Find(&rows).Error
	return rows, err
}

func applySiteProfile(inbound *model.Inbound, profile website.Config) error {
	stream, err := website.Parse(inbound.StreamSettings)
	if err != nil {
		return err
	}
	mode := stream.Website.Mode
	if mode == "" || mode == "auto" {
		switch {
		case stream.Security == "reality":
			mode = "reality"
		case stream.Security == "tls" && (stream.Network == "tcp" || stream.Network == "raw") && (inbound.Protocol == model.VLESS || inbound.Protocol == model.Trojan):
			mode = "fallback"
		case stream.Security == "tls" && (stream.Network == "xhttp" || stream.Network == "ws" || stream.Network == "grpc" || stream.Network == "httpupgrade"):
			mode = "path"
		default:
			mode = "independent"
		}
	}
	if profile.Standalone && mode != "independent" && profile.Port == inbound.Port {
		return errors.New("disable the website independent listener before sharing this inbound port")
	}
	profile.Mode = mode
	var raw map[string]any
	if err := json.Unmarshal([]byte(inbound.StreamSettings), &raw); err != nil {
		return err
	}
	if raw == nil {
		raw = map[string]any{}
	}
	if mode == "path" || mode == "fallback" {
		tlsSettings, _ := raw["tlsSettings"].(map[string]any)
		if tlsSettings == nil {
			tlsSettings = map[string]any{}
		}
		tlsSettings["serverName"] = profile.Domain
		if profile.AutoTLS {
			tlsSettings["certificates"] = []any{}
		} else {
			tlsSettings["certificates"] = []any{map[string]any{"certificateFile": profile.CertificateFile, "keyFile": profile.KeyFile}}
		}
		if mode == "fallback" {
			tlsSettings["alpn"] = []string{"http/1.1"}
		}
		raw["tlsSettings"] = tlsSettings
		inbound.ShareAddrStrategy = "custom"
		inbound.ShareAddr = profile.Domain
	}
	if mode == "reality" {
		rs, _ := raw["realitySettings"].(map[string]any)
		if rs == nil {
			rs = map[string]any{}
		}
		rs["serverNames"] = []string{profile.Domain}
		rs["target"] = website.SiteAddress(inbound.Tag)
		rs["xver"] = 0
		raw["realitySettings"] = rs
	}
	if mode == "independent" && profile.Port == inbound.Port && inbound.Protocol != model.Hysteria && inbound.Protocol != model.TUIC && inbound.Protocol != model.WireGuard && inbound.Protocol != model.AmneziaWG {
		return errors.New("this protocol cannot share the same TCP port with an HTTPS website; choose another website port or a compatible TLS transport")
	}
	raw["website"] = profile
	encoded, _ := json.Marshal(raw)
	inbound.StreamSettings = string(encoded)
	return nil
}

func resolveInboundSite(inbound *model.Inbound) error {
	stream, err := website.Parse(inbound.StreamSettings)
	if err != nil {
		return err
	}
	if !stream.Website.Enabled || stream.Website.SiteID == 0 {
		return nil
	}
	var row model.DecoySite
	if err := database.GetDB().First(&row, stream.Website.SiteID).Error; err != nil {
		return fmt.Errorf("associated website does not exist: %w", err)
	}
	var profile website.Config
	if err := json.Unmarshal([]byte(row.Config), &profile); err != nil {
		return err
	}
	profile.SiteID = row.Id
	profile.Enabled = true
	return applySiteProfile(inbound, profile)
}

func (s *DecoySiteService) Save(row *model.DecoySite) error {
	row.Name = strings.TrimSpace(row.Name)
	if row.Name == "" || len(row.Name) > 160 {
		return errors.New("website name is required and must be at most 160 bytes")
	}
	var c website.Config
	if err := json.Unmarshal([]byte(row.Config), &c); err != nil {
		return err
	}
	c.Enabled = true
	c.Mode = "independent"
	c.Domain = strings.ToLower(strings.TrimSpace(c.Domain))
	if c.Port == 0 {
		c.Port = 443
	}
	if !website.ValidTemplate(c.Template) {
		return errors.New("select an installed website template")
	}
	if err := (website.Stream{Website: c}).Validate("", c.Port); err != nil {
		return err
	}
	encoded, _ := json.Marshal(c)
	row.Config = string(encoded)
	changed := false
	err := database.GetDB().Transaction(func(tx *gorm.DB) error {
		if row.Id != 0 {
			var existing model.DecoySite
			if err := tx.First(&existing, row.Id).Error; err != nil {
				return err
			}
		}
		if err := tx.Save(row).Error; err != nil {
			return err
		}
		var inbounds []model.Inbound
		if err := tx.Find(&inbounds).Error; err != nil {
			return err
		}
		for _, ib := range inbounds {
			stream, err := website.Parse(ib.StreamSettings)
			if err != nil || !stream.Website.Enabled || stream.Website.SiteID != row.Id {
				continue
			}
			profile := c
			profile.SiteID = row.Id
			profile.Mode = stream.Website.Mode
			if err := applySiteProfile(&ib, profile); err != nil {
				return err
			}
			if err := validateInboundWebsite(&ib); err != nil {
				return err
			}
			if err := tx.Model(&ib).Updates(map[string]any{"stream_settings": ib.StreamSettings, "share_addr": ib.ShareAddr, "share_addr_strategy": ib.ShareAddrStrategy}).Error; err != nil {
				return err
			}
			changed = true
		}
		return nil
	})
	if err == nil && changed {
		(&XrayService{}).SetToNeedReconcile()
		InvalidateSubData()
	}
	return err
}

func (s *DecoySiteService) Delete(id int) error {
	return database.GetDB().Transaction(func(tx *gorm.DB) error {
		var inbounds []model.Inbound
		if err := tx.Find(&inbounds).Error; err != nil {
			return err
		}
		for _, ib := range inbounds {
			stream, _ := website.Parse(ib.StreamSettings)
			if stream.Website.SiteID == id {
				return errors.New("website is referenced by an inbound; remove its association first")
			}
		}
		return tx.Delete(&model.DecoySite{}, id).Error
	})
}

func (s *DecoySiteService) ListInboundReferences() ([]model.Inbound, error) {
	var rows []model.Inbound
	err := database.GetDB().Select("id", "tag", "stream_settings").Find(&rows).Error
	return rows, err
}
