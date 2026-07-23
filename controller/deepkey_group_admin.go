package controller

import (
	"math"
	"net/http"
	"sort"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

type deepKeyGroupAdminStatus struct {
	Group                 string   `json:"group"`
	CatalogPresent        bool     `json:"catalog_present"`
	Configured            bool     `json:"configured"`
	CatalogRatio          *float64 `json:"catalog_ratio,omitempty"`
	ConfiguredRatio       *float64 `json:"configured_ratio,omitempty"`
	ChannelCount          int      `json:"channel_count"`
	EnabledChannelCount   int      `json:"enabled_channel_count"`
	DisabledChannelCount  int      `json:"disabled_channel_count"`
	ModelCount            int      `json:"model_count"`
	TokenCount            int64    `json:"token_count"`
	KeyFingerprint        string   `json:"key_fingerprint"`
	KeyConfigurationValid bool     `json:"key_configuration_valid"`
	LastTestTime          int64    `json:"last_test_time"`
	ResponseTime          int      `json:"response_time"`
	Issues                []string `json:"issues"`
}

func buildDeepKeyGroupAdminStatuses(
	catalog *deepKeyPricingCatalog,
	channelStatuses map[string]model.DeepKeyChannelGroupStatus,
	configuredRatios map[string]float64,
	tokenCounts map[string]int64,
) []deepKeyGroupAdminStatus {
	names := make(map[string]struct{}, len(channelStatuses)+len(configuredRatios))
	if catalog != nil {
		for name := range catalog.GroupRatio {
			names[name] = struct{}{}
		}
	}
	for name := range channelStatuses {
		names[name] = struct{}{}
	}
	for name := range configuredRatios {
		names[name] = struct{}{}
	}
	result := make([]deepKeyGroupAdminStatus, 0, len(names))
	for name := range names {
		status := deepKeyGroupAdminStatus{Group: name, Issues: []string{}}
		if channelStatus, ok := channelStatuses[name]; ok {
			status.ChannelCount = channelStatus.ChannelCount
			status.EnabledChannelCount = channelStatus.EnabledChannelCount
			status.DisabledChannelCount = channelStatus.DisabledChannelCount
			status.ModelCount = channelStatus.ModelCount
			status.TokenCount = channelStatus.TokenCount
			status.KeyFingerprint = channelStatus.KeyFingerprint
			status.KeyConfigurationValid = channelStatus.KeyConfigurationValid
			status.LastTestTime = channelStatus.LastTestTime
			status.ResponseTime = channelStatus.ResponseTime
		}
		if count, ok := tokenCounts[name]; ok {
			status.TokenCount = count
		}
		if ratio, ok := configuredRatios[name]; ok {
			ratioCopy := ratio
			status.Configured = true
			status.ConfiguredRatio = &ratioCopy
		}
		if catalog != nil {
			if ratio, ok := catalog.GroupRatio[name]; ok {
				ratioCopy := ratio
				status.CatalogPresent = true
				status.CatalogRatio = &ratioCopy
			}
		}

		if catalog != nil {
			if !status.CatalogPresent && status.Configured {
				status.Issues = append(status.Issues, "not_in_catalog")
			}
			if status.CatalogPresent && !status.Configured {
				status.Issues = append(status.Issues, "missing_configuration")
			}
		}
		if status.ChannelCount == 0 {
			status.Issues = append(status.Issues, "missing_channel")
		} else if status.EnabledChannelCount == 0 {
			status.Issues = append(status.Issues, "no_enabled_channel")
		}
		if status.ChannelCount > 0 && !status.KeyConfigurationValid {
			status.Issues = append(status.Issues, "invalid_key_configuration")
		}
		if catalog != nil && status.CatalogRatio != nil && status.ConfiguredRatio != nil &&
			math.Abs(*status.CatalogRatio-*status.ConfiguredRatio) > 0.000001 {
			status.Issues = append(status.Issues, "ratio_drift")
		}
		result = append(result, status)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Group < result[j].Group })
	return result
}

func GetDeepKeyGroupAdminStatuses(c *gin.Context) {
	catalog, err := getDeepKeyPricingCatalog()
	catalogAvailable := err == nil
	catalogError := ""
	if err != nil {
		catalogError = err.Error()
	}
	channelStatuses, err := model.GetDeepKeyChannelGroupStatuses()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	configuredRatios := ratio_setting.GetGroupRatioCopy()
	groupNames := make(map[string]struct{}, len(channelStatuses)+len(configuredRatios))
	for name := range channelStatuses {
		groupNames[name] = struct{}{}
	}
	for name := range configuredRatios {
		groupNames[name] = struct{}{}
	}
	if catalog != nil {
		for name := range catalog.GroupRatio {
			groupNames[name] = struct{}{}
		}
	}
	groups := make([]string, 0, len(groupNames))
	for name := range groupNames {
		groups = append(groups, name)
	}
	tokenCounts, err := model.CountTokensByGroups(groups)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	statuses := buildDeepKeyGroupAdminStatuses(catalog, channelStatuses, configuredRatios, tokenCounts)
	c.JSON(http.StatusOK, gin.H{
		"success":           true,
		"message":           "",
		"data":              statuses,
		"catalog_available": catalogAvailable,
		"catalog_error":     catalogError,
	})
}
