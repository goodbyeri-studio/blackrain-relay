package controller

import (
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

const deepKeyMaxGroupRatio = 1000

type deepKeyGroupSyncData struct {
	GroupRatio       map[string]float64 `json:"group_ratio"`
	UserUsableGroups map[string]string  `json:"user_usable_groups"`
	AutoGroups       []string           `json:"auto_groups"`
	Count            int                `json:"count"`
}

func buildDeepKeyGroupSyncData(catalog *deepKeyPricingCatalog, enabledChannelGroups map[string]struct{}) (*deepKeyGroupSyncData, error) {
	if catalog == nil || len(catalog.GroupRatio) == 0 {
		return nil, fmt.Errorf("DeepKey pricing returned no groups")
	}

	groupRatio := make(map[string]float64, len(catalog.GroupRatio))
	userUsableGroups := make(map[string]string, len(catalog.GroupRatio))
	for rawName, ratio := range catalog.GroupRatio {
		name := strings.TrimSpace(rawName)
		if name == "" {
			continue
		}
		if ratio <= 0 || math.IsNaN(ratio) || math.IsInf(ratio, 0) || ratio > deepKeyMaxGroupRatio {
			return nil, fmt.Errorf("DeepKey group %q ratio must be within (0, %d]", name, deepKeyMaxGroupRatio)
		}
		if _, enabled := enabledChannelGroups[name]; !enabled {
			continue
		}

		description := strings.TrimSpace(catalog.UsableGroup[rawName])
		if description == "" {
			description = name
		}
		groupRatio[name] = ratio
		userUsableGroups[name] = description
	}
	if len(groupRatio) == 0 {
		return nil, fmt.Errorf("DeepKey pricing has no groups backed by enabled local channels")
	}

	autoGroups := make([]string, 0, len(catalog.AutoGroups))
	seenAutoGroups := make(map[string]struct{}, len(catalog.AutoGroups))
	for _, rawName := range catalog.AutoGroups {
		name := strings.TrimSpace(rawName)
		if _, exists := groupRatio[name]; !exists {
			continue
		}
		if _, exists := seenAutoGroups[name]; exists {
			continue
		}
		autoGroups = append(autoGroups, name)
		seenAutoGroups[name] = struct{}{}
	}

	return &deepKeyGroupSyncData{
		GroupRatio:       groupRatio,
		UserUsableGroups: userUsableGroups,
		AutoGroups:       autoGroups,
		Count:            len(groupRatio),
	}, nil
}

func SyncDeepKeyGroups(c *gin.Context) {
	catalog, err := refreshDeepKeyPricingCatalog()
	if err != nil {
		common.ApiErrorMsg(c, "获取 DeepKey 分组失败: "+err.Error())
		return
	}

	enabledChannelGroups, err := model.GetEnabledChannelGroups()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	data, err := buildDeepKeyGroupSyncData(catalog, enabledChannelGroups)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	groupRatioJSON, err := common.Marshal(data.GroupRatio)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	userUsableGroupsJSON, err := common.Marshal(data.UserUsableGroups)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	autoGroupsJSON, err := common.Marshal(data.AutoGroups)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	if err := model.UpdateOptionsBulk(map[string]string{
		"GroupRatio":       string(groupRatioJSON),
		"UserUsableGroups": string(userUsableGroupsJSON),
		"AutoGroups":       string(autoGroupsJSON),
	}); err != nil {
		common.ApiError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    data,
	})
}
