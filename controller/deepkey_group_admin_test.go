package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDeepKeyGroupAdminStatusesReportsLifecycleGaps(t *testing.T) {
	catalog := &deepKeyPricingCatalog{GroupRatio: map[string]float64{
		"healthy":         1.3,
		"missing-channel": 2.6,
		"missing-config":  3.9,
	}}
	channelStatuses := map[string]model.DeepKeyChannelGroupStatus{
		"healthy": {
			Group: "healthy", ChannelCount: 1, EnabledChannelCount: 1,
			ModelCount: 2, TokenCount: 4, KeyFingerprint: "0123456789abcdef", KeyConfigurationValid: true,
		},
		"local-only": {
			Group: "local-only", ChannelCount: 1, DisabledChannelCount: 1,
			ModelCount: 1, KeyFingerprint: "fedcba9876543210", KeyConfigurationValid: true,
		},
		"invalid-key": {
			Group: "invalid-key", ChannelCount: 1, EnabledChannelCount: 1,
		},
	}
	configuredRatios := map[string]float64{
		"healthy":         1.3,
		"missing-channel": 3,
		"local-only":      1,
		"invalid-key":     1,
	}
	tokenCounts := map[string]int64{
		"healthy":         4,
		"missing-channel": 2,
		"local-only":      3,
	}

	statuses := buildDeepKeyGroupAdminStatuses(catalog, channelStatuses, configuredRatios, tokenCounts)
	require.Len(t, statuses, 5)
	byGroup := make(map[string]deepKeyGroupAdminStatus, len(statuses))
	for _, status := range statuses {
		byGroup[status.Group] = status
	}
	assert.Empty(t, byGroup["healthy"].Issues)
	assert.Equal(t, int64(4), byGroup["healthy"].TokenCount)
	assert.ElementsMatch(t, []string{"missing_channel", "ratio_drift"}, byGroup["missing-channel"].Issues)
	assert.ElementsMatch(t, []string{"missing_configuration", "missing_channel"}, byGroup["missing-config"].Issues)
	assert.ElementsMatch(t, []string{"not_in_catalog", "no_enabled_channel"}, byGroup["local-only"].Issues)
	assert.ElementsMatch(t, []string{"not_in_catalog", "invalid_key_configuration"}, byGroup["invalid-key"].Issues)
	assert.Equal(t, int64(3), byGroup["local-only"].TokenCount)
}

func TestBuildDeepKeyGroupAdminStatusesKeepsLocalDataWithoutCatalog(t *testing.T) {
	statuses := buildDeepKeyGroupAdminStatuses(
		nil,
		map[string]model.DeepKeyChannelGroupStatus{},
		map[string]float64{"configured-only": 1.3},
		map[string]int64{"configured-only": 2},
	)

	require.Len(t, statuses, 1)
	assert.Equal(t, "configured-only", statuses[0].Group)
	assert.True(t, statuses[0].Configured)
	assert.Equal(t, int64(2), statuses[0].TokenCount)
	assert.Equal(t, []string{"missing_channel"}, statuses[0].Issues)
}
