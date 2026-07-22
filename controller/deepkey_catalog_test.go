package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyDeepKeyCatalogMarkup(t *testing.T) {
	items := []model.Pricing{
		{ModelName: "token-model", QuotaType: 0, ModelRatio: 0.25, CompletionRatio: 4},
		{ModelName: "request-model", QuotaType: 1, ModelPrice: 0.08},
	}

	applyDeepKeyCatalogMarkup(items, 30)

	require.Len(t, items, 2)
	assert.Equal(t, 0.325, items[0].ModelRatio)
	assert.Equal(t, 4.0, items[0].CompletionRatio)
	assert.Equal(t, 0.104, items[1].ModelPrice)
	for _, item := range items {
		assert.True(t, item.CatalogOnly)
		assert.Equal(t, "DeepKey", item.CatalogSource)
	}
}

func TestMergePricingCatalogKeepsLocalModel(t *testing.T) {
	local := []model.Pricing{{ModelName: "shared", ModelRatio: 9}}
	catalog := []model.Pricing{
		{ModelName: "shared", ModelRatio: 1, CatalogOnly: true},
		{ModelName: "catalog", ModelRatio: 2, CatalogOnly: true},
	}

	merged := mergePricingCatalog(local, catalog)

	require.Len(t, merged, 2)
	assert.Equal(t, "shared", merged[0].ModelName)
	assert.Equal(t, 9.0, merged[0].ModelRatio)
	assert.False(t, merged[0].CatalogOnly)
	assert.Equal(t, "catalog", merged[1].ModelName)
	assert.True(t, merged[1].CatalogOnly)
}

func TestMergePricingVendorsKeepsLocalVendor(t *testing.T) {
	local := []model.PricingVendor{{ID: 3, Name: "Local OpenAI"}}
	catalog := []model.PricingVendor{
		{ID: 3, Name: "OpenAI"},
		{ID: 4, Name: "Google"},
	}

	merged := mergePricingVendors(local, catalog)

	require.Len(t, merged, 2)
	assert.Equal(t, "Local OpenAI", merged[0].Name)
	assert.Equal(t, "Google", merged[1].Name)
}
