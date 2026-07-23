package model

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const DeepKeyPricingMigrationOption = "DeepKeyPricingMigration"

var ErrDeepKeyPricingMigrationStale = errors.New("DeepKey migration preview is stale")

type DeepKeyPricingMigrationSnapshot struct {
	ModelRatio map[string]float64 `json:"model_ratio"`
	ModelPrice map[string]float64 `json:"model_price"`
	GroupRatio map[string]float64 `json:"group_ratio"`
	Migration  string             `json:"migration"`
}

type DeepKeyChannelGroupStatus struct {
	Group                 string `json:"group"`
	ChannelCount          int    `json:"channel_count"`
	EnabledChannelCount   int    `json:"enabled_channel_count"`
	DisabledChannelCount  int    `json:"disabled_channel_count"`
	ModelCount            int    `json:"model_count"`
	TokenCount            int64  `json:"token_count"`
	KeyFingerprint        string `json:"key_fingerprint"`
	KeyConfigurationValid bool   `json:"key_configuration_valid"`
	LastTestTime          int64  `json:"last_test_time"`
	ResponseTime          int    `json:"response_time"`
}

func (snapshot DeepKeyPricingMigrationSnapshot) Hash() (string, error) {
	encoded, err := common.Marshal(snapshot)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:]), nil
}

func readPricingOption(tx *gorm.DB, key string) (string, error) {
	var option Option
	err := lockForUpdate(tx).Where(&Option{Key: key}).First(&option).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return option.Value, nil
}

func decodePricingMap(value string) (map[string]float64, error) {
	result := make(map[string]float64)
	if strings.TrimSpace(value) == "" {
		return result, nil
	}
	if err := common.Unmarshal([]byte(value), &result); err != nil {
		return nil, err
	}
	return result, nil
}

func loadDeepKeyPricingMigrationSnapshot(tx *gorm.DB) (DeepKeyPricingMigrationSnapshot, error) {
	groupRatioJSON, err := readPricingOption(tx, "GroupRatio")
	if err != nil {
		return DeepKeyPricingMigrationSnapshot{}, err
	}
	modelRatioJSON, err := readPricingOption(tx, "ModelRatio")
	if err != nil {
		return DeepKeyPricingMigrationSnapshot{}, err
	}
	modelPriceJSON, err := readPricingOption(tx, "ModelPrice")
	if err != nil {
		return DeepKeyPricingMigrationSnapshot{}, err
	}
	migration, err := readPricingOption(tx, DeepKeyPricingMigrationOption)
	if err != nil {
		return DeepKeyPricingMigrationSnapshot{}, err
	}
	modelRatio, err := decodePricingMap(modelRatioJSON)
	if err != nil {
		return DeepKeyPricingMigrationSnapshot{}, fmt.Errorf("decode ModelRatio: %w", err)
	}
	modelPrice, err := decodePricingMap(modelPriceJSON)
	if err != nil {
		return DeepKeyPricingMigrationSnapshot{}, fmt.Errorf("decode ModelPrice: %w", err)
	}
	groupRatio, err := decodePricingMap(groupRatioJSON)
	if err != nil {
		return DeepKeyPricingMigrationSnapshot{}, fmt.Errorf("decode GroupRatio: %w", err)
	}
	return DeepKeyPricingMigrationSnapshot{
		ModelRatio: modelRatio,
		ModelPrice: modelPrice,
		GroupRatio: groupRatio,
		Migration:  migration,
	}, nil
}

func GetDeepKeyPricingMigrationSnapshot() (DeepKeyPricingMigrationSnapshot, error) {
	return loadDeepKeyPricingMigrationSnapshot(DB)
}

func IsDeepKeyBaseURL(baseURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	return err == nil && strings.EqualFold(parsed.Hostname(), "deepkey.top")
}

func validateDeepKeyChannelSet(channels []Channel) (map[string]struct{}, error) {
	deepKeyGroups := make(map[string]struct{})
	nonDeepKeyGroups := make(map[string]struct{})
	groupKeys := make(map[string]string)

	for i := range channels {
		channel := &channels[i]
		if channel.Status != common.ChannelStatusEnabled {
			continue
		}
		groups := channel.GetGroups()
		if !IsDeepKeyBaseURL(channel.GetBaseURL()) {
			for _, group := range groups {
				nonDeepKeyGroups[group] = struct{}{}
			}
			continue
		}

		keys := channel.GetKeys()
		upstreamKeys := make([]string, 0, len(keys))
		for _, key := range keys {
			key = strings.TrimSpace(key)
			if key != "" {
				upstreamKeys = append(upstreamKeys, key)
			}
		}
		if len(upstreamKeys) != 1 {
			return nil, fmt.Errorf("DeepKey channel %d must contain exactly one upstream key", channel.Id)
		}
		upstreamKey := upstreamKeys[0]
		for _, group := range groups {
			if existingKey, exists := groupKeys[group]; exists && existingKey != upstreamKey {
				return nil, fmt.Errorf("DeepKey group %q has multiple upstream key configurations", group)
			}
			groupKeys[group] = upstreamKey
			deepKeyGroups[group] = struct{}{}
		}
	}

	for group := range deepKeyGroups {
		if _, exists := nonDeepKeyGroups[group]; exists {
			return nil, fmt.Errorf("DeepKey group %q is also assigned to a non-DeepKey channel", group)
		}
	}
	return deepKeyGroups, nil
}

func GetEnabledDeepKeyChannelGroups() (map[string]struct{}, error) {
	var channels []Channel
	if err := DB.Where("status = ?", common.ChannelStatusEnabled).Find(&channels).Error; err != nil {
		return nil, err
	}
	return validateDeepKeyChannelSet(channels)
}

func GetDeepKeyChannelGroupStatuses() (map[string]DeepKeyChannelGroupStatus, error) {
	var channels []Channel
	if err := DB.Find(&channels).Error; err != nil {
		return nil, err
	}

	statuses := make(map[string]DeepKeyChannelGroupStatus)
	modelsByGroup := make(map[string]map[string]struct{})
	keysByGroup := make(map[string]map[string]struct{})
	keyConfigurationValidByGroup := make(map[string]bool)
	for i := range channels {
		channel := &channels[i]
		if !IsDeepKeyBaseURL(channel.GetBaseURL()) {
			continue
		}
		keys := make([]string, 0)
		for _, key := range channel.GetKeys() {
			key = strings.TrimSpace(key)
			if key != "" {
				keys = append(keys, key)
			}
		}
		for _, group := range channel.GetGroups() {
			group = strings.TrimSpace(group)
			if group == "" {
				continue
			}
			status := statuses[group]
			status.Group = group
			status.ChannelCount++
			if channel.Status == common.ChannelStatusEnabled {
				status.EnabledChannelCount++
			} else {
				status.DisabledChannelCount++
			}
			if channel.TestTime > status.LastTestTime {
				status.LastTestTime = channel.TestTime
				status.ResponseTime = channel.ResponseTime
			}
			statuses[group] = status

			if modelsByGroup[group] == nil {
				modelsByGroup[group] = make(map[string]struct{})
			}
			for _, modelName := range channel.GetModels() {
				modelName = strings.TrimSpace(modelName)
				if modelName != "" {
					modelsByGroup[group][modelName] = struct{}{}
				}
			}
			if keysByGroup[group] == nil {
				keysByGroup[group] = make(map[string]struct{})
				keyConfigurationValidByGroup[group] = true
			}
			if len(keys) != 1 {
				keyConfigurationValidByGroup[group] = false
			}
			for _, key := range keys {
				keysByGroup[group][key] = struct{}{}
			}
		}
	}

	tokenCounts, err := CountTokensByGroups(nil)
	if err != nil {
		return nil, err
	}
	for group, status := range statuses {
		status.ModelCount = len(modelsByGroup[group])
		status.TokenCount = tokenCounts[group]
		if len(keysByGroup[group]) == 1 {
			for key := range keysByGroup[group] {
				digest := sha256.Sum256([]byte(key))
				status.KeyFingerprint = hex.EncodeToString(digest[:])[:16]
			}
		}
		status.KeyConfigurationValid = keyConfigurationValidByGroup[group] && len(keysByGroup[group]) == 1
		statuses[group] = status
	}
	return statuses, nil
}

func CountTokensByGroups(groups []string) (map[string]int64, error) {
	return countTokensByGroups(DB, groups)
}

func countTokensByGroups(tx *gorm.DB, groups []string) (map[string]int64, error) {
	type tokenGroupCount struct {
		Group string `gorm:"column:group_name"`
		Count int64  `gorm:"column:token_count"`
	}
	var counts []tokenGroupCount
	query := tx.Model(&Token{}).
		Select(commonGroupCol + " AS group_name, COUNT(*) AS token_count").
		Where(commonGroupCol + " <> ''")
	if len(groups) > 0 {
		query = query.Where(commonGroupCol+" IN ?", groups)
	}
	query = query.Clauses(clause.GroupBy{Columns: []clause.Column{{Name: "group"}}})
	if err := query.Find(&counts).Error; err != nil {
		return nil, err
	}
	result := make(map[string]int64, len(counts))
	for _, count := range counts {
		result[count.Group] = count.Count
	}
	return result, nil
}

func ValidateDeepKeyChannelGroupIsolation(candidate *Channel) error {
	if candidate == nil || candidate.Status != common.ChannelStatusEnabled {
		return nil
	}
	var channels []Channel
	query := DB.Where("status = ?", common.ChannelStatusEnabled)
	if candidate.Id > 0 {
		query = query.Where("id <> ?", candidate.Id)
	}
	if err := query.Find(&channels).Error; err != nil {
		return err
	}
	channels = append(channels, *candidate)
	_, err := validateDeepKeyChannelSet(channels)
	return err
}

func ValidateDeepKeyChannelsForEnable(ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	var channels []Channel
	if err := DB.Where("status = ? OR id IN ?", common.ChannelStatusEnabled, ids).Find(&channels).Error; err != nil {
		return err
	}
	requested := make(map[int]struct{}, len(ids))
	for _, id := range ids {
		requested[id] = struct{}{}
	}
	for i := range channels {
		if _, exists := requested[channels[i].Id]; exists {
			channels[i].Status = common.ChannelStatusEnabled
		}
	}
	_, err := validateDeepKeyChannelSet(channels)
	return err
}

func GetEnabledDeepKeyModelNames() ([]string, error) {
	var channels []Channel
	if err := DB.Select("type", "base_url", "models").Where("status = ?", common.ChannelStatusEnabled).Find(&channels).Error; err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	for _, channel := range channels {
		if !IsDeepKeyBaseURL(channel.GetBaseURL()) {
			continue
		}
		for _, modelName := range channel.GetModels() {
			modelName = strings.TrimSpace(modelName)
			if modelName != "" {
				seen[modelName] = struct{}{}
			}
		}
	}
	models := make([]string, 0, len(seen))
	for modelName := range seen {
		models = append(models, modelName)
	}
	sort.Strings(models)
	return models, nil
}

func ApplyDeepKeyPricingMigration(expectedHash string, modelRatio, modelPrice, groupRatio map[string]float64, sourceMarker string) error {
	modelRatioJSON, err := common.Marshal(modelRatio)
	if err != nil {
		return err
	}
	modelPriceJSON, err := common.Marshal(modelPrice)
	if err != nil {
		return err
	}
	groupRatioJSON, err := common.Marshal(groupRatio)
	if err != nil {
		return err
	}
	markerJSON, err := common.Marshal(map[string]string{
		"version": "v1",
		"source":  "deepkey",
		"marker":  sourceMarker,
	})
	if err != nil {
		return err
	}
	values := map[string]string{
		"ModelRatio":                  string(modelRatioJSON),
		"ModelPrice":                  string(modelPriceJSON),
		"GroupRatio":                  string(groupRatioJSON),
		DeepKeyPricingMigrationOption: string(markerJSON),
	}

	if err := DB.Transaction(func(tx *gorm.DB) error {
		current, err := loadDeepKeyPricingMigrationSnapshot(tx)
		if err != nil {
			return err
		}
		currentHash, err := current.Hash()
		if err != nil {
			return err
		}
		if currentHash != expectedHash {
			return ErrDeepKeyPricingMigrationStale
		}
		return updateOptionsBulkTx(tx, values)
	}); err != nil {
		return err
	}
	for key, value := range values {
		if err := updateOptionMap(key, value); err != nil {
			return err
		}
	}
	return nil
}
