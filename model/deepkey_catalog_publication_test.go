package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSyncDeepKeyCatalogModelsPreservesAdminPublicationOverride(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Channel{}, &Ability{}, &Model{}))
	const liveModel = "zz-deepkey-catalog-live"
	const retiredModel = "zz-deepkey-catalog-retired"
	const channelName = "zz-deepkey-catalog-channel"
	deepKeyURL := "https://deepkey.top"
	require.NoError(t, DB.Unscoped().Where("model_name IN ?", []string{liveModel, retiredModel}).Delete(&Model{}).Error)
	require.NoError(t, DB.Where("model IN ?", []string{liveModel, retiredModel}).Delete(&Ability{}).Error)
	require.NoError(t, DB.Unscoped().Where("name = ?", channelName).Delete(&Channel{}).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Unscoped().Where("model_name IN ?", []string{liveModel, retiredModel}).Delete(&Model{}).Error)
		require.NoError(t, DB.Where("model IN ?", []string{liveModel, retiredModel}).Delete(&Ability{}).Error)
		require.NoError(t, DB.Unscoped().Where("name = ?", channelName).Delete(&Channel{}).Error)
	})
	channel := Channel{
		Name: channelName, Key: "test-key", BaseURL: &deepKeyURL,
		Status: common.ChannelStatusEnabled, Models: liveModel + "," + retiredModel,
	}
	require.NoError(t, DB.Create(&channel).Error)
	require.NoError(t, DB.Create(&Ability{
		Group: "live-group", Model: liveModel, ChannelId: channel.Id, Enabled: true,
	}).Error)
	items := []DeepKeyCatalogItem{
		{ModelName: liveModel, VendorID: 7, EnableGroups: []string{"live-group"}},
		{ModelName: retiredModel, VendorID: 7, EnableGroups: []string{"retired-group"}},
	}

	first, err := SyncDeepKeyCatalogModels(items)
	require.NoError(t, err)
	assert.Equal(t, 2, first.Created)
	assert.Equal(t, 1, first.Available)
	assert.Equal(t, 1, first.Unavailable)

	var live Model
	var retired Model
	require.NoError(t, DB.Where("model_name = ?", liveModel).First(&live).Error)
	require.NoError(t, DB.Where("model_name = ?", retiredModel).First(&retired).Error)
	assert.Equal(t, 1, live.Status)
	assert.True(t, live.UpstreamAvailable)
	assert.Equal(t, 0, retired.Status)
	assert.False(t, retired.UpstreamAvailable)
	assert.ErrorIs(t, UpdateModelPublicationStatus(retired.Id, 1), ErrCatalogModelUpstreamUnavailable)

	require.NoError(t, DB.Model(&Ability{}).Where("model = ?", liveModel).Update("enabled", false).Error)
	changed, err := SyncDeepKeyCatalogModels(items)
	require.NoError(t, err)
	assert.Equal(t, 2, changed.Unavailable)
	require.NoError(t, DB.Model(&Ability{}).Where("model = ?", liveModel).Update("enabled", true).Error)
	_, err = SyncDeepKeyCatalogModels(items)
	require.NoError(t, err)

	require.NoError(t, UpdateModelPublicationStatus(live.Id, 0))
	second, err := SyncDeepKeyCatalogModels(items)
	require.NoError(t, err)
	assert.Zero(t, second.Updated)
	require.NoError(t, DB.Where("id = ?", live.Id).First(&live).Error)
	assert.Equal(t, CatalogPublishBlocked, live.PublishOverride)
	assert.Equal(t, 0, live.Status)

	pricing := []Pricing{{ModelName: liveModel, EnableGroup: []string{"live-group"}}}
	filtered, err := FilterPublishedDeepKeyCatalog(pricing)
	require.NoError(t, err)
	assert.Empty(t, filtered)

	require.NoError(t, UpdateModelPublicationStatus(live.Id, 1))
	filtered, err = FilterPublishedDeepKeyCatalog(pricing)
	require.NoError(t, err)
	assert.Equal(t, pricing, filtered)
}

func TestDeepKeyCatalogAvailabilityRequiresMatchingEnabledAbility(t *testing.T) {
	availability := &deepKeyCatalogAvailability{modelsByGroup: map[string]map[string]struct{}{
		"live-group": {"live-model": {}},
	}}

	tests := []struct {
		name   string
		model  string
		groups []string
		want   bool
	}{
		{name: "matching model and group", model: "live-model", groups: []string{"live-group"}, want: true},
		{name: "same group different model", model: "other-model", groups: []string{"live-group"}},
		{name: "all with matching model", model: "live-model", groups: []string{"all"}, want: true},
		{name: "all without matching model", model: "other-model", groups: []string{"all"}},
		{name: "empty groups", model: "live-model"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, availability.hasModel(test.model, test.groups))
		})
	}
}

func TestFilterPublishedDeepKeyCatalogFailsClosedForUnknownModel(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Model{}))
	const unknownModel = "zz-deepkey-catalog-unknown"
	require.NoError(t, DB.Unscoped().Where("model_name = ?", unknownModel).Delete(&Model{}).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Unscoped().Where("model_name = ?", unknownModel).Delete(&Model{}).Error)
	})

	filtered, err := FilterPublishedDeepKeyCatalog([]Pricing{{ModelName: unknownModel, EnableGroup: []string{"live-group"}}})

	require.NoError(t, err)
	assert.Empty(t, filtered)
}
