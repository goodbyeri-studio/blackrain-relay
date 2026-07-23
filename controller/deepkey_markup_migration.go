package controller

import (
	"errors"
	"fmt"
	"net/http"
	"sort"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

type deepKeyPricingMigrationRequest struct {
	Apply        bool   `json:"apply"`
	SnapshotHash string `json:"snapshot_hash"`
}

type deepKeyPricingModelChange struct {
	ModelName string  `json:"model_name"`
	Before    float64 `json:"before"`
	After     float64 `json:"after"`
	Present   bool    `json:"present"`
}

type deepKeyPricingGroupChange struct {
	GroupName string  `json:"group_name"`
	Before    float64 `json:"before"`
	After     float64 `json:"after"`
	Present   bool    `json:"present"`
}

type deepKeyPricingMigrationPlan struct {
	Version             string                      `json:"version"`
	SnapshotHash        string                      `json:"snapshot_hash"`
	ConfirmedModelCount int                         `json:"confirmed_model_count"`
	ModelRatioChanges   []deepKeyPricingModelChange `json:"model_ratio_changes"`
	ModelPriceChanges   []deepKeyPricingModelChange `json:"model_price_changes"`
	GroupRatioChanges   []deepKeyPricingGroupChange `json:"group_ratio_changes"`
	ConflictCount       int                         `json:"conflict_count"`
	ChangedCount        int                         `json:"changed_count"`
	DesiredModelRatio   map[string]float64          `json:"-"`
	DesiredModelPrice   map[string]float64          `json:"-"`
	DesiredGroupRatio   map[string]float64          `json:"-"`
}

func copyPricingMap(source map[string]float64) map[string]float64 {
	copy := make(map[string]float64, len(source))
	for key, value := range source {
		copy[key] = value
	}
	return copy
}

func buildDeepKeyPricingMigrationPlan(catalog *deepKeyPricingCatalog, snapshot model.DeepKeyPricingMigrationSnapshot, confirmedModels []string, enabledChannelGroups map[string]struct{}) (*deepKeyPricingMigrationPlan, error) {
	if catalog == nil {
		return nil, fmt.Errorf("DeepKey pricing returned no catalog")
	}
	snapshotHash, err := snapshot.Hash()
	if err != nil {
		return nil, err
	}
	confirmed := make(map[string]struct{}, len(confirmedModels))
	for _, modelName := range confirmedModels {
		confirmed[modelName] = struct{}{}
	}

	plan := &deepKeyPricingMigrationPlan{
		Version:             "v1",
		SnapshotHash:        snapshotHash,
		ConfirmedModelCount: len(confirmed),
		DesiredModelRatio:   copyPricingMap(snapshot.ModelRatio),
		DesiredModelPrice:   copyPricingMap(snapshot.ModelPrice),
		DesiredGroupRatio:   copyPricingMap(snapshot.GroupRatio),
	}
	for _, item := range catalog.Models {
		if _, confirmedModel := confirmed[item.ModelName]; !confirmedModel {
			continue
		}
		if item.QuotaType == 1 {
			plan.DesiredModelPrice[item.ModelName] = item.ModelPrice
			continue
		}
		plan.DesiredModelRatio[item.ModelName] = item.ModelRatio
	}

	groupData, err := buildDeepKeyGroupSyncData(catalog, enabledChannelGroups)
	if err != nil {
		return nil, err
	}
	for groupName, ratio := range groupData.GroupRatio {
		plan.DesiredGroupRatio[groupName] = ratio
	}

	for _, modelName := range sortedMapKeys(plan.DesiredModelRatio) {
		if _, confirmedModel := confirmed[modelName]; !confirmedModel {
			continue
		}
		before, present := snapshot.ModelRatio[modelName]
		after := plan.DesiredModelRatio[modelName]
		if !present || !nearlyEqual(before, after) {
			plan.ModelRatioChanges = append(plan.ModelRatioChanges, deepKeyPricingModelChange{ModelName: modelName, Before: before, After: after, Present: present})
		}
	}
	for _, modelName := range sortedMapKeys(plan.DesiredModelPrice) {
		if _, confirmedModel := confirmed[modelName]; !confirmedModel {
			continue
		}
		before, present := snapshot.ModelPrice[modelName]
		after := plan.DesiredModelPrice[modelName]
		if !present || !nearlyEqual(before, after) {
			plan.ModelPriceChanges = append(plan.ModelPriceChanges, deepKeyPricingModelChange{ModelName: modelName, Before: before, After: after, Present: present})
		}
	}
	for _, groupName := range sortedMapKeys(groupData.GroupRatio) {
		before, present := snapshot.GroupRatio[groupName]
		after := plan.DesiredGroupRatio[groupName]
		if !present || !nearlyEqual(before, after) {
			plan.GroupRatioChanges = append(plan.GroupRatioChanges, deepKeyPricingGroupChange{GroupName: groupName, Before: before, After: after, Present: present})
			if present {
				plan.ConflictCount++
			}
		}
	}
	plan.ChangedCount = len(plan.ModelRatioChanges) + len(plan.ModelPriceChanges) + len(plan.GroupRatioChanges)
	return plan, nil
}

func sortedMapKeys(values map[string]float64) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func MigrateDeepKeyPricing(c *gin.Context) {
	var request deepKeyPricingMigrationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		common.ApiErrorMsg(c, "请求参数格式错误")
		return
	}
	catalog, err := refreshDeepKeyPricingCatalog()
	if err != nil {
		common.ApiErrorMsg(c, "获取 DeepKey 最新定价失败: "+err.Error())
		return
	}
	snapshot, err := model.GetDeepKeyPricingMigrationSnapshot()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	confirmedModels, err := model.GetEnabledDeepKeyModelNames()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	enabledGroups, err := model.GetEnabledChannelGroups()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	plan, err := buildDeepKeyPricingMigrationPlan(catalog, snapshot, confirmedModels, enabledGroups)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	if !request.Apply {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": plan})
		return
	}
	if request.SnapshotHash == "" || request.SnapshotHash != plan.SnapshotHash {
		c.JSON(http.StatusConflict, gin.H{"success": false, "message": "迁移预览已过期，请重新预览", "data": plan})
		return
	}
	if err := model.ApplyDeepKeyPricingMigration(plan.SnapshotHash, plan.DesiredModelRatio, plan.DesiredModelPrice, plan.DesiredGroupRatio, plan.SnapshotHash); err != nil {
		if errors.Is(err, model.ErrDeepKeyPricingMigrationStale) {
			c.JSON(http.StatusConflict, gin.H{"success": false, "message": "迁移预览已过期，请重新预览"})
			return
		}
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "DeepKey 30% 分组倍率迁移完成", "data": plan})
}
