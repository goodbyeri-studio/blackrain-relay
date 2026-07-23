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

func GetEnabledDeepKeyModelNames() ([]string, error) {
	var channels []Channel
	if err := DB.Select("type", "base_url", "models").Where("status = ?", common.ChannelStatusEnabled).Find(&channels).Error; err != nil {
		return nil, err
	}
	seen := make(map[string]struct{})
	for _, channel := range channels {
		parsed, err := url.Parse(channel.GetBaseURL())
		if err != nil || !strings.EqualFold(parsed.Hostname(), "deepkey.top") {
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
