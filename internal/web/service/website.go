package service

import (
	"encoding/json"
	"errors"
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
	if len(masks["tcp"]) > 0 || len(masks["udp"]) > 0 {
		return errors.New("built-in websites cannot use transport masks")
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
	return site.Validate(string(inbound.Protocol), inbound.Port)
}

func (s *XrayService) ReconcileWebsites() error {
	lock.Lock()
	defer lock.Unlock()
	if isManuallyStopped.Load() {
		website.GetManager().StopAll()
		return nil
	}
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
	for _, ib := range inbounds {
		site, err := website.Parse(ib.StreamSettings)
		if err != nil || !site.Website.Enabled {
			continue
		}
		if err := validateInboundWebsite(ib); err != nil {
			return nil, err
		}
		desired = append(desired, website.Instance{Tag: ib.Tag, Listen: ib.Listen, Port: ib.Port, Stream: site})
	}
	return desired, nil
}
