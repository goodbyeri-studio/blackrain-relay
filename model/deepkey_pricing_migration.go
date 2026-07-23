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
)

const DeepKeyPricingMigrationOption = "DeepKeyPricingMigration"

var ErrDeepKeyPricingMigrationStale = errors.New("DeepKey migration preview is stale")

type DeepKeyPricingMigrationSnapshot struct {
	ModelRatio map[string]float64 `json:"model_ratio"`
	ModelPrice map[string]float64 `json:"model_price"`
	GroupRatio map[string]float64 `json:"group_ratio"`
	Migration  string             `json:"migration"`
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
	modelRatioJSON, err := readPricingOption(tx, "ModelRatio")
	if err != nil {
		return DeepKeyPricingMigrationSnapshot{}, err
	}
	modelPriceJSON, err := readPricingOption(tx, "ModelPrice")
	if err != nil {
		return DeepKeyPricingMigrationSnapshot{}, err
	}
	groupRatioJSON, err := readPricingOption(tx, "GroupRatio")
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

func writePricingOption(tx *gorm.DB, key, value string) error {
	option := Option{Key: key}
	if err := tx.Where(&Option{Key: key}).FirstOrCreate(&option).Error; err != nil {
		return err
	}
	option.Value = value
	return tx.Save(&option).Error
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
		for key, value := range values {
			if err := writePricingOption(tx, key, value); err != nil {
				return err
			}
		}
		return nil
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
