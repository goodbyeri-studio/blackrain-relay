package controller

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDeepKeyPricingCatalogServesStaleWhileRefreshing(t *testing.T) {
	originalFetcher := deepKeyCatalogFetcher
	defer func() {
		deepKeyCatalogFetcher = originalFetcher
		deepKeyCatalogCache.Lock()
		deepKeyCatalogCache.catalog = nil
		deepKeyCatalogCache.fetchedAt = time.Time{}
		deepKeyCatalogCache.Unlock()
	}()

	stale := &deepKeyPricingCatalog{Models: []model.Pricing{{ModelName: "stale"}}}
	refreshed := &deepKeyPricingCatalog{Models: []model.Pricing{{ModelName: "fresh"}}}
	started := make(chan struct{})
	release := make(chan struct{})
	deepKeyCatalogFetcher = func() (*deepKeyPricingCatalog, error) {
		close(started)
		<-release
		return refreshed, nil
	}
	deepKeyCatalogCache.Lock()
	deepKeyCatalogCache.catalog = stale
	deepKeyCatalogCache.fetchedAt = time.Now().Add(-deepKeyCatalogCacheTTL)
	deepKeyCatalogCache.Unlock()

	start := time.Now()
	catalog, err := getDeepKeyPricingCatalog()
	require.NoError(t, err)
	assert.Same(t, stale, catalog)
	assert.Less(t, time.Since(start), 500*time.Millisecond)

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("stale catalog did not start a background refresh")
	}
	close(release)

	assert.Eventually(t, func() bool {
		deepKeyCatalogCache.RLock()
		defer deepKeyCatalogCache.RUnlock()
		return deepKeyCatalogCache.catalog == refreshed
	}, time.Second, 10*time.Millisecond)
}

func TestRefreshDeepKeyPricingCatalogWaitsForFreshCatalog(t *testing.T) {
	originalFetcher := deepKeyCatalogFetcher
	defer func() {
		deepKeyCatalogFetcher = originalFetcher
		deepKeyCatalogCache.Lock()
		deepKeyCatalogCache.catalog = nil
		deepKeyCatalogCache.fetchedAt = time.Time{}
		deepKeyCatalogCache.Unlock()
	}()

	stale := &deepKeyPricingCatalog{Models: []model.Pricing{{ModelName: "stale"}}}
	fresh := &deepKeyPricingCatalog{Models: []model.Pricing{{ModelName: "fresh"}}}
	deepKeyCatalogFetcher = func() (*deepKeyPricingCatalog, error) {
		return fresh, nil
	}
	deepKeyCatalogCache.Lock()
	deepKeyCatalogCache.catalog = stale
	deepKeyCatalogCache.fetchedAt = time.Now()
	deepKeyCatalogCache.Unlock()

	catalog, err := refreshDeepKeyPricingCatalog()
	require.NoError(t, err)
	assert.Same(t, fresh, catalog)
	deepKeyCatalogCache.RLock()
	defer deepKeyCatalogCache.RUnlock()
	assert.Same(t, fresh, deepKeyCatalogCache.catalog)
}

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
