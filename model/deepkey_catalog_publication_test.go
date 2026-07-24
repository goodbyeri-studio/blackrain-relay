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

	require.NoError(t, UpdateModelPublicationStatus(live.Id, 0))
	second, err := SyncDeepKeyCatalogModels(items)
	require.NoError(t, err)
	assert.Zero(t, second.Updated)
	require.NoError(t, DB.Where("id = ?", live.Id).First(&live).Error)
	assert.Equal(t, CatalogPublishBlocked, live.PublishOverride)
	assert.Equal(t, 0, live.Status)

	filtered, err := FilterPublishedDeepKeyCatalog([]Pricing{{ModelName: liveModel}})
	require.NoError(t, err)
	assert.Empty(t, filtered)

	require.NoError(t, UpdateModelPublicationStatus(live.Id, 1))
	filtered, err = FilterPublishedDeepKeyCatalog([]Pricing{{ModelName: liveModel}})
	require.NoError(t, err)
	assert.Equal(t, []Pricing{{ModelName: liveModel}}, filtered)
}
