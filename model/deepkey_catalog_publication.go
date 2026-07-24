package model

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

var ErrCatalogModelUpstreamUnavailable = errors.New("catalog model is unavailable upstream")

const (
	CatalogPublishInherit = 0
	CatalogPublishForce   = 1
	CatalogPublishBlocked = -1
)

type DeepKeyCatalogItem struct {
	ModelName    string
	Description  string
	Icon         string
	Tags         string
	VendorID     int
	EnableGroups []string
}

type DeepKeyCatalogSyncResult struct {
	Total       int `json:"total"`
	Available   int `json:"available"`
	Unavailable int `json:"unavailable"`
	Created     int `json:"created"`
	Updated     int `json:"updated"`
}

// GetAllDeepKeyModelNames returns model names configured on both enabled and
// disabled DeepKey channels. Disabled channels are included so a model that
// disappears from the upstream catalog can still be explicitly unpublished.
func GetAllDeepKeyModelNames() ([]string, error) {
	var channels []Channel
	if err := DB.Select("base_url", "models").Find(&channels).Error; err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	for _, channel := range channels {
		parsed, err := url.Parse(channel.GetBaseURL())
		if err != nil || !strings.EqualFold(parsed.Hostname(), "deepkey.top") {
			continue
		}
		for _, name := range channel.GetModels() {
			name = strings.TrimSpace(name)
			if name != "" {
				seen[name] = struct{}{}
			}
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names, nil
}

// SyncDeepKeyCatalogModels persists the upstream availability of DeepKey
// models. It deliberately leaves PublishOverride untouched so an admin's
// manual unpublish survives a later upstream refresh.
func SyncDeepKeyCatalogModels(items []DeepKeyCatalogItem) (DeepKeyCatalogSyncResult, error) {
	if DB == nil {
		return DeepKeyCatalogSyncResult{}, errors.New("database is not initialized")
	}
	knownNames, err := GetAllDeepKeyModelNames()
	if err != nil {
		return DeepKeyCatalogSyncResult{}, err
	}
	byName := make(map[string]DeepKeyCatalogItem, len(items))
	availableNames := make(map[string]struct{}, len(items))
	for _, item := range items {
		if name := strings.TrimSpace(item.ModelName); name != "" {
			byName[name] = item
			availableNames[name] = struct{}{}
		}
	}
	for _, name := range knownNames {
		if _, ok := byName[name]; !ok {
			byName[name] = DeepKeyCatalogItem{ModelName: name}
		}
	}
	enabledGroups, err := GetEnabledDeepKeyGroups()
	if err != nil {
		return DeepKeyCatalogSyncResult{}, err
	}

	result := DeepKeyCatalogSyncResult{Total: len(byName)}
	err = DB.Transaction(func(tx *gorm.DB) error {
		now := common.GetTimestamp()
		var existing []Model
		names := make([]string, 0, len(byName))
		for name := range byName {
			names = append(names, name)
		}
		if err := tx.Where("catalog_only = ? OR model_name IN ?", true, names).Find(&existing).Error; err != nil {
			return err
		}
		existingByName := make(map[string]*Model, len(existing))
		for i := range existing {
			existingByName[existing[i].ModelName] = &existing[i]
		}

		for name, item := range byName {
			_, available := availableNames[name]
			if available && len(item.EnableGroups) > 0 && !hasEnabledDeepKeyGroup(item.EnableGroups, enabledGroups) {
				available = false
			}
			meta := existingByName[name]
			if meta == nil {
				status := 0
				if available {
					status = 1
				}
				meta = &Model{
					ModelName: name, Description: item.Description, Icon: item.Icon,
					Tags: item.Tags, VendorID: item.VendorID, NameRule: NameRuleExact,
					Status: status, SyncOfficial: 0, CatalogOnly: true,
					CatalogSource: "DeepKey", UpstreamAvailable: available,
					CreatedTime: now, UpdatedTime: now,
				}
				if err := tx.Create(meta).Error; err != nil {
					return err
				}
				// GORM applies default tags to zero values during Create.
				if err := tx.Model(&Model{}).Where("id = ?", meta.Id).Updates(map[string]interface{}{
					"status":        status,
					"sync_official": 0,
					"created_time":  now,
					"updated_time":  now,
				}).Error; err != nil {
					return err
				}
				existingByName[name] = meta
				result.Created++
				if available {
					result.Available++
				} else {
					result.Unavailable++
				}
				continue
			}
			if !meta.CatalogOnly {
				continue
			}
			description, icon, tags, vendorID := meta.Description, meta.Icon, meta.Tags, meta.VendorID
			createdTime := meta.CreatedTime
			if createdTime == 0 {
				createdTime = now
			}
			if available {
				description, icon, tags, vendorID = item.Description, item.Icon, item.Tags, item.VendorID
				result.Available++
			} else {
				result.Unavailable++
			}
			status := 1
			if meta.PublishOverride == CatalogPublishBlocked || !available {
				status = 0
			}
			if meta.Description == description && meta.Icon == icon && meta.Tags == tags &&
				meta.VendorID == vendorID && meta.CatalogSource == "DeepKey" &&
				meta.UpstreamAvailable == available && meta.Status == status &&
				meta.SyncOfficial == 0 && meta.CreatedTime != 0 {
				continue
			}
			if err := tx.Model(&Model{}).Where("id = ?", meta.Id).Updates(map[string]interface{}{
				"description":        description,
				"icon":               icon,
				"tags":               tags,
				"vendor_id":          vendorID,
				"catalog_only":       true,
				"catalog_source":     "DeepKey",
				"upstream_available": available,
				"status":             status,
				"sync_official":      0,
				"created_time":       createdTime,
				"updated_time":       now,
			}).Error; err != nil {
				return err
			}
			result.Updated++
		}

		// Existing catalog records no longer present in channels or the catalog
		// are also marked unavailable, including records from older syncs.
		for _, meta := range existing {
			if _, ok := byName[meta.ModelName]; ok {
				continue
			}
			if !meta.UpstreamAvailable && meta.Status == 0 {
				result.Unavailable++
				continue
			}
			if err := tx.Model(&Model{}).Where("id = ?", meta.Id).Updates(map[string]interface{}{
				"upstream_available": false,
				"status":             0,
			}).Error; err != nil {
				return err
			}
			result.Unavailable++
			result.Updated++
		}
		return nil
	})
	return result, err
}

func GetEnabledDeepKeyGroups() (map[string]struct{}, error) {
	var channels []Channel
	if err := DB.Select("id", "base_url").Where("status = ?", common.ChannelStatusEnabled).Find(&channels).Error; err != nil {
		return nil, err
	}
	channelIDs := make([]int, 0, len(channels))
	for _, channel := range channels {
		parsed, err := url.Parse(channel.GetBaseURL())
		if err == nil && strings.EqualFold(parsed.Hostname(), "deepkey.top") {
			channelIDs = append(channelIDs, channel.Id)
		}
	}
	groups := make(map[string]struct{})
	if len(channelIDs) == 0 {
		return groups, nil
	}
	var rows []struct {
		GroupName string
	}
	if err := DB.Table("abilities").Select(commonGroupCol+" AS group_name").Where("channel_id IN ? AND enabled = ?", channelIDs, true).Distinct().Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		group := row.GroupName
		if group = strings.TrimSpace(group); group != "" {
			groups[group] = struct{}{}
		}
	}
	return groups, nil
}

func hasEnabledDeepKeyGroup(groups []string, enabled map[string]struct{}) bool {
	for _, group := range groups {
		if group == "all" {
			return true
		}
		if _, ok := enabled[group]; ok {
			return true
		}
	}
	return false
}

func FilterPublishedDeepKeyCatalog(items []Pricing) ([]Pricing, error) {
	if DB == nil || len(items) == 0 {
		return items, nil
	}
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.ModelName)
	}
	var models []Model
	if err := DB.Select("model_name", "status", "catalog_only", "upstream_available").Where("model_name IN ?", names).Find(&models).Error; err != nil {
		return nil, err
	}
	state := make(map[string]Model, len(models))
	for _, meta := range models {
		state[meta.ModelName] = meta
	}
	filtered := make([]Pricing, 0, len(items))
	for _, item := range items {
		meta, ok := state[item.ModelName]
		if !ok || (meta.Status == 1 && (!meta.CatalogOnly || meta.UpstreamAvailable)) {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func UpdateModelPublicationStatus(id, status int) error {
	if status != 0 && status != 1 {
		return fmt.Errorf("invalid model status: %d", status)
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var meta Model
		if err := lockForUpdate(tx).First(&meta, id).Error; err != nil {
			return err
		}
		updates := map[string]interface{}{
			"status":       status,
			"updated_time": common.GetTimestamp(),
		}
		if meta.CatalogOnly {
			if status == 1 && !meta.UpstreamAvailable {
				return ErrCatalogModelUpstreamUnavailable
			}
			if status == 1 {
				updates["publish_override"] = CatalogPublishForce
			} else {
				updates["publish_override"] = CatalogPublishBlocked
			}
		}
		return tx.Model(&Model{}).Where("id = ?", id).Updates(updates).Error
	})
}
