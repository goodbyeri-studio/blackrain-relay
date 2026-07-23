package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedDefaultCNYSubscriptionPlansIsIdempotent(t *testing.T) {
	truncateTables(t)

	require.NoError(t, seedDefaultCNYSubscriptionPlans())
	require.NoError(t, seedDefaultCNYSubscriptionPlans())

	var plans []SubscriptionPlan
	require.NoError(t, DB.Order("sort_order desc").Find(&plans).Error)
	require.Len(t, plans, len(defaultCNYSubscriptionPlans))
	assert.Equal(t, "轻量包", plans[0].Title)
	assert.Equal(t, "CNY", plans[0].Currency)
	assert.Equal(t, float64(10), plans[0].PriceAmount)
	assert.Greater(t, plans[0].TotalAmount, int64(0))
}

func TestSeedDefaultCNYSubscriptionPlansPreservesAdminCatalog(t *testing.T) {
	truncateTables(t)

	customPlan := &SubscriptionPlan{
		Title:            "管理员自定义套餐",
		PriceAmount:      88,
		Currency:         "CNY",
		DurationUnit:     SubscriptionDurationMonth,
		DurationValue:    1,
		Enabled:          true,
		QuotaResetPeriod: SubscriptionResetNever,
	}
	require.NoError(t, DB.Create(customPlan).Error)
	require.NoError(t, seedDefaultCNYSubscriptionPlans())

	var plans []SubscriptionPlan
	require.NoError(t, DB.Find(&plans).Error)
	require.Len(t, plans, 1)
	assert.Equal(t, customPlan.Title, plans[0].Title)
}
