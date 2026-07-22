package controller

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"golang.org/x/sync/singleflight"
)

const (
	deepKeyPricingCatalogURL     = "https://deepkey.top/api/pricing"
	deepKeyPricingMarkupPercent  = 30.0
	deepKeyCatalogCacheTTL       = 15 * time.Minute
	deepKeyCatalogRequestTimeout = 8 * time.Second
	deepKeyCatalogMaxBodyBytes   = 5 << 20
)

type deepKeyPricingCatalog struct {
	Models            []model.Pricing
	Vendors           []model.PricingVendor
	GroupRatio        map[string]float64
	UsableGroup       map[string]string
	SupportedEndpoint map[string]common.EndpointInfo
	AutoGroups        []string
}

type deepKeyPricingCatalogResponse struct {
	Success           bool                           `json:"success"`
	Data              []model.Pricing                `json:"data"`
	Vendors           []model.PricingVendor          `json:"vendors"`
	GroupRatio        map[string]float64             `json:"group_ratio"`
	UsableGroup       map[string]string              `json:"usable_group"`
	SupportedEndpoint map[string]common.EndpointInfo `json:"supported_endpoint"`
	AutoGroups        []string                       `json:"auto_groups"`
}

var deepKeyCatalogCache = struct {
	sync.RWMutex
	catalog   *deepKeyPricingCatalog
	fetchedAt time.Time
}{}

var (
	deepKeyCatalogRefresh singleflight.Group
	deepKeyCatalogFetcher = fetchDeepKeyPricingCatalog
)

func applyDeepKeyCatalogMarkup(items []model.Pricing, markupPercent float64) {
	multiplier := 1 + markupPercent/100
	for i := range items {
		items[i].CatalogOnly = true
		items[i].CatalogSource = "DeepKey"
		if items[i].QuotaType == 1 {
			items[i].ModelPrice = roundRatioValue(items[i].ModelPrice * multiplier)
			continue
		}
		items[i].ModelRatio = roundRatioValue(items[i].ModelRatio * multiplier)
	}
}

func fetchDeepKeyPricingCatalog() (*deepKeyPricingCatalog, error) {
	ctx, cancel := context.WithTimeout(context.Background(), deepKeyCatalogRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, deepKeyPricingCatalogURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := service.GetHttpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("DeepKey pricing returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, deepKeyCatalogMaxBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > deepKeyCatalogMaxBodyBytes {
		return nil, fmt.Errorf("DeepKey pricing response exceeds %d bytes", deepKeyCatalogMaxBodyBytes)
	}
	body = normalizePricingResponseJSON(body)

	var upstream deepKeyPricingCatalogResponse
	if err := common.Unmarshal(body, &upstream); err != nil {
		return nil, err
	}
	if !upstream.Success || len(upstream.Data) == 0 {
		return nil, fmt.Errorf("DeepKey pricing returned no models")
	}

	applyDeepKeyCatalogMarkup(upstream.Data, deepKeyPricingMarkupPercent)
	return &deepKeyPricingCatalog{
		Models:            upstream.Data,
		Vendors:           upstream.Vendors,
		GroupRatio:        upstream.GroupRatio,
		UsableGroup:       upstream.UsableGroup,
		SupportedEndpoint: upstream.SupportedEndpoint,
		AutoGroups:        upstream.AutoGroups,
	}, nil
}

func getDeepKeyPricingCatalog() (*deepKeyPricingCatalog, error) {
	deepKeyCatalogCache.RLock()
	catalog := deepKeyCatalogCache.catalog
	fresh := catalog != nil && time.Since(deepKeyCatalogCache.fetchedAt) < deepKeyCatalogCacheTTL
	deepKeyCatalogCache.RUnlock()
	if fresh {
		return catalog, nil
	}

	// Keep serving a stale catalog while one request refreshes it. The
	// singleflight call deduplicates concurrent refreshes without holding the
	// cache lock during the remote request.
	if catalog != nil {
		go func() {
			_, _ = refreshDeepKeyPricingCatalog()
		}()
		return catalog, nil
	}

	return refreshDeepKeyPricingCatalog()
}

func refreshDeepKeyPricingCatalog() (*deepKeyPricingCatalog, error) {
	value, err, _ := deepKeyCatalogRefresh.Do("pricing", func() (any, error) {
		freshCatalog, fetchErr := deepKeyCatalogFetcher()
		if fetchErr != nil {
			return nil, fetchErr
		}
		deepKeyCatalogCache.Lock()
		deepKeyCatalogCache.catalog = freshCatalog
		deepKeyCatalogCache.fetchedAt = time.Now()
		deepKeyCatalogCache.Unlock()
		return freshCatalog, nil
	})
	if err != nil {
		return nil, err
	}
	refreshed, ok := value.(*deepKeyPricingCatalog)
	if !ok {
		return nil, fmt.Errorf("DeepKey pricing cache returned an invalid value")
	}
	return refreshed, nil
}

func mergePricingCatalog(local, catalog []model.Pricing) []model.Pricing {
	merged := make([]model.Pricing, 0, len(local)+len(catalog))
	seen := make(map[string]struct{}, len(local)+len(catalog))
	for _, item := range local {
		merged = append(merged, item)
		seen[item.ModelName] = struct{}{}
	}
	for _, item := range catalog {
		if _, exists := seen[item.ModelName]; exists {
			continue
		}
		merged = append(merged, item)
		seen[item.ModelName] = struct{}{}
	}
	return merged
}

func mergePricingVendors(local, catalog []model.PricingVendor) []model.PricingVendor {
	merged := make([]model.PricingVendor, 0, len(local)+len(catalog))
	seen := make(map[int]struct{}, len(local)+len(catalog))
	for _, vendor := range local {
		merged = append(merged, vendor)
		seen[vendor.ID] = struct{}{}
	}
	for _, vendor := range catalog {
		if _, exists := seen[vendor.ID]; exists {
			continue
		}
		merged = append(merged, vendor)
		seen[vendor.ID] = struct{}{}
	}
	return merged
}
