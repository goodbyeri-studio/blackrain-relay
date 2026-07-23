package controller

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDeepKeyGroupSyncData(t *testing.T) {
	catalog := &deepKeyPricingCatalog{
		GroupRatio: map[string]float64{
			"default": 1,
			"claude":  0.4,
			" ":       2,
		},
		UsableGroup: map[string]string{
			"":        "用户分组",
			"default": "默认分组",
			"claude":  "claude-code混合高可用",
		},
		AutoGroups: []string{"default", "missing", "default"},
	}
	enabledChannelGroups := map[string]struct{}{
		"claude": {},
	}

	data, err := buildDeepKeyGroupSyncData(catalog, enabledChannelGroups)
	require.NoError(t, err)
	assert.Equal(t, map[string]float64{"claude": 0.4}, data.GroupRatio)
	assert.Equal(t, map[string]string{
		"claude": "claude-code混合高可用",
	}, data.UserUsableGroups)
	assert.Empty(t, data.AutoGroups)
	assert.Equal(t, 1, data.Count)
}

func TestBuildDeepKeyGroupSyncDataRejectsUnsafeRatios(t *testing.T) {
	testCases := []struct {
		name  string
		ratio float64
	}{
		{name: "negative", ratio: -1},
		{name: "zero", ratio: 0},
		{name: "above maximum", ratio: deepKeyMaxGroupRatio + 1},
		{name: "nan", ratio: math.NaN()},
		{name: "positive infinity", ratio: math.Inf(1)},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := buildDeepKeyGroupSyncData(&deepKeyPricingCatalog{
				GroupRatio: map[string]float64{"broken": testCase.ratio},
			}, map[string]struct{}{"broken": {}})

			require.Error(t, err)
			assert.Contains(t, err.Error(), "ratio must be within")
		})
	}
}
