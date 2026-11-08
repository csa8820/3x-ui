package database

import (
	"encoding/json"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// retiredSiteTemplates maps the English templates removed in favour of the
// built-in Chinese ones to the closest replacement.
var retiredSiteTemplates = map[string]string{
	"studio": "cloud", "journal": "news", "portfolio": "brand", "business": "group", "minimal": "cloud",
	"sb-agency": "group", "sb-clean-blog": "news", "sb-freelancer": "brand", "sb-landing-page": "cloud",
	"sb-creative": "brand", "sb-grayscale": "cloud", "sb-resume": "legal", "sb-business-casual": "brand",
	"sb-stylish-portfolio": "brand",
}

func replaceRetiredTemplate(site map[string]any) bool {
	id, _ := site["template"].(string)
	replacement, ok := retiredSiteTemplates[id]
	if ok {
		site["template"] = replacement
	}
	return ok
}

// Idempotent, not seeder-gated: a restored backup can bring retired IDs back.
func migrateRetiredSiteTemplates() error {
	return db.Transaction(func(tx *gorm.DB) error {
		var sites []model.DecoySite
		if err := tx.Find(&sites).Error; err != nil {
			return err
		}
		for _, site := range sites {
			var config map[string]any
			if json.Unmarshal([]byte(site.Config), &config) != nil || !replaceRetiredTemplate(config) {
				continue
			}
			encoded, _ := json.Marshal(config)
			if err := tx.Model(&site).Update("config", string(encoded)).Error; err != nil {
				return err
			}
		}
		var inbounds []model.Inbound
		if err := tx.Select("id", "stream_settings").Where("stream_settings LIKE ?", `%"website"%`).Find(&inbounds).Error; err != nil {
			return err
		}
		for _, inbound := range inbounds {
			var stream map[string]any
			if json.Unmarshal([]byte(inbound.StreamSettings), &stream) != nil {
				continue
			}
			site, _ := stream["website"].(map[string]any)
			if site == nil || !replaceRetiredTemplate(site) {
				continue
			}
			encoded, _ := json.Marshal(stream)
			if err := tx.Model(&model.Inbound{}).Where("id = ?", inbound.Id).Update("stream_settings", string(encoded)).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
