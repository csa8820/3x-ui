package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/website"
)

func validateInboundWebsite(inbound *model.Inbound) error {
	site, err := website.Parse(inbound.StreamSettings)
	if err != nil {
		return err
	}
	if !site.Website.Enabled {
		return nil
	}
	var raw map[string]json.RawMessage
	_ = json.Unmarshal([]byte(inbound.StreamSettings), &raw)
	var masks map[string][]json.RawMessage
	_ = json.Unmarshal(raw["finalmask"], &masks)
	if site.Website.Mode != "independent" && (len(masks["tcp"]) > 0 || len(masks["udp"]) > 0) {
		return errors.New("built-in websites cannot use transport masks")
	}
	if site.Website.Mode == "fallback" && inbound.Protocol == model.VLESS {
		var settings struct {
			Decryption string `json:"decryption"`
		}
		_ = json.Unmarshal([]byte(inbound.Settings), &settings)
		if settings.Decryption != "" && settings.Decryption != "none" {
			return errors.New("VLESS Encryption cannot use RAW fallback; use an HTTP transport, REALITY or an independent website")
		}
	}
	if inbound.NodeID != nil {
		return errors.New("built-in websites must be configured on the hosting panel directly")
	}
	if strings.HasPrefix(inbound.Listen, "@") || strings.HasPrefix(inbound.Listen, "/") {
		return errors.New("website public listener requires a TCP IP address")
	}
	if inbound.Listen != "" && net.ParseIP(inbound.Listen) == nil {
		return errors.New("website listen address must be an IP address or empty")
	}
	if !website.ValidTemplate(site.Website.Template) {
		return errors.New("website template is not installed")
	}
	port := inbound.Port
	if site.Website.Mode == "independent" && site.Website.Port != 0 {
		port = site.Website.Port
	}
	return site.Validate(string(inbound.Protocol), port)
}

func (s *XrayService) ReconcileWebsites() error {
	lock.Lock()
	defer lock.Unlock()

	desired, err := desiredWebsites()
	if err != nil {
		return err
	}
	return website.GetManager().Reconcile(desired)
}

func desiredWebsites() ([]website.Instance, error) {
	var inbounds []*model.Inbound
	err := database.GetDB().Select("tag", "listen", "port", "protocol", "stream_settings").
		Where("enable = ? AND node_id IS NULL", true).Find(&inbounds).Error
	if err != nil {
		return nil, err
	}
	var desired []website.Instance
	seen := map[string]bool{}
	var profiles []model.DecoySite
	if err := database.GetDB().Find(&profiles).Error; err != nil {
		return nil, err
	}
	for _, row := range profiles {
		var profile website.Config
		if json.Unmarshal([]byte(row.Config), &profile) != nil || !profile.Standalone {
			continue
		}
		profile.SiteID = row.Id
		profile.Mode = "independent"
		tag := fmt.Sprintf("site-%d", row.Id)
		desired = append(desired, website.Instance{Tag: tag, Listen: profile.Listen, Port: profile.Port, Stream: website.Stream{Website: profile}})
		seen[tag] = true
	}
	if isManuallyStopped.Load() {
		return desired, nil
	}
	for _, ib := range inbounds {
		site, err := website.Parse(ib.StreamSettings)
		if err != nil || !site.Website.Enabled {
			continue
		}
		if err := validateInboundWebsite(ib); err != nil {
			return nil, err
		}
		inst := WebsiteInstance(ib, site)
		if !seen[inst.Tag] {
			desired = append(desired, inst)
			seen[inst.Tag] = true
		}
	}
	return desired, nil
}

func WebsiteInstance(ib *model.Inbound, site website.Stream) website.Instance {
	inst := website.Instance{Tag: ib.Tag, Listen: ib.Listen, Port: ib.Port, Stream: site}
	if site.Website.Mode == "independent" {
		inst.Listen = site.Website.Listen
		inst.Port = site.Website.Port
		if inst.Port == 0 {
			inst.Port = 443
		}
		if site.Website.SiteID > 0 {
			inst.Tag = fmt.Sprintf("site-%d", site.Website.SiteID)
		}
	}
	return inst
}
