package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildDeepKeyPricingMigrationPlanMovesMarkupFromModelToGroup(t *testing.T) {
	snapshot := model.DeepKeyPricingMigrationSnapshot{
		ModelRatio: map[string]float64{"gpt": 1.3, "manual": 7},
		ModelPrice: map[string]float64{"image": 1.04},
		GroupRatio: map[string]float64{"deepkey": 1},
	}
	catalog := &deepKeyPricingCatalog{
		Models: []model.Pricing{
			{ModelName: "gpt", ModelRatio: 1},
			{ModelName: "image", QuotaType: 1, ModelPrice: 0.8},
			{ModelName: "manual", ModelRatio: 2},
		},
		GroupRatio: map[string]float64{"deepkey": 1.3},
	}

	plan, err := buildDeepKeyPricingMigrationPlan(catalog, snapshot, []string{"gpt", "image"}, map[string]struct{}{"deepkey": {}})
	require.NoError(t, err)
	assert.Equal(t, 1.0, plan.DesiredModelRatio["gpt"])
	assert.Equal(t, 0.8, plan.DesiredModelPrice["image"])
	assert.Equal(t, 1.3, plan.DesiredGroupRatio["deepkey"])
	assert.Equal(t, 7.0, plan.DesiredModelRatio["manual"])
	assert.Len(t, plan.ModelRatioChanges, 1)
	assert.Len(t, plan.ModelPriceChanges, 1)
	assert.Len(t, plan.GroupRatioChanges, 1)
	assert.Equal(t, 1, plan.ConflictCount)
}

func TestBuildDeepKeyPricingMigrationPlanDoesNotTouchUnconfirmedSameName(t *testing.T) {
	snapshot := model.DeepKeyPricingMigrationSnapshot{
		ModelRatio: map[string]float64{"shared": 7},
		GroupRatio: map[string]float64{"deepkey": 1},
	}
	plan, err := buildDeepKeyPricingMigrationPlan(&deepKeyPricingCatalog{
		Models:     []model.Pricing{{ModelName: "shared", ModelRatio: 1}},
		GroupRatio: map[string]float64{"deepkey": 1.3},
	}, snapshot, nil, map[string]struct{}{"deepkey": {}})
	require.NoError(t, err)
	assert.Equal(t, 7.0, plan.DesiredModelRatio["shared"])
	assert.Empty(t, plan.ModelRatioChanges)
}
