package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyDeepKeyPricingMigrationIsAtomicAndIdempotent(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Option{}))
	common.OptionMapRWMutex.Lock()
	originalOptionMap := common.OptionMap
	common.OptionMap = make(map[string]string)
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptionMap
		common.OptionMapRWMutex.Unlock()
	})
	for _, key := range []string{"ModelRatio", "ModelPrice", "GroupRatio", DeepKeyPricingMigrationOption} {
		t.Cleanup(func() { DB.Where("key = ?", key).Delete(&Option{}) })
	}
	require.NoError(t, DB.Create(&Option{Key: "ModelRatio", Value: `{"gpt":1.3,"manual":7}`}).Error)
	require.NoError(t, DB.Create(&Option{Key: "ModelPrice", Value: `{"image":1.04}`}).Error)
	require.NoError(t, DB.Create(&Option{Key: "GroupRatio", Value: `{"deepkey":1}`}).Error)

	before, err := GetDeepKeyPricingMigrationSnapshot()
	require.NoError(t, err)
	beforeHash, err := before.Hash()
	require.NoError(t, err)
	desiredRatio := map[string]float64{"gpt": 1, "manual": 7}
	desiredPrice := map[string]float64{"image": 0.8}
	desiredGroups := map[string]float64{"deepkey": 1.3}
	require.NoError(t, ApplyDeepKeyPricingMigration(beforeHash, desiredRatio, desiredPrice, desiredGroups, "preview-1"))

	after, err := GetDeepKeyPricingMigrationSnapshot()
	require.NoError(t, err)
	assert.Equal(t, desiredRatio, after.ModelRatio)
	assert.Equal(t, desiredPrice, after.ModelPrice)
	assert.Equal(t, desiredGroups, after.GroupRatio)
	assert.Contains(t, after.Migration, `"version":"v1"`)
	assert.Contains(t, after.Migration, `"source":"deepkey"`)

	afterHash, err := after.Hash()
	require.NoError(t, err)
	require.NoError(t, ApplyDeepKeyPricingMigration(afterHash, desiredRatio, desiredPrice, desiredGroups, "preview-2"))
	repeated, err := GetDeepKeyPricingMigrationSnapshot()
	require.NoError(t, err)
	assert.Equal(t, desiredRatio, repeated.ModelRatio)
	assert.Equal(t, desiredGroups, repeated.GroupRatio)

	err = ApplyDeepKeyPricingMigration(beforeHash, map[string]float64{"gpt": 99}, desiredPrice, desiredGroups, "stale")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "stale")
	unchanged, err := GetDeepKeyPricingMigrationSnapshot()
	require.NoError(t, err)
	assert.Equal(t, desiredRatio, unchanged.ModelRatio)
}

func TestGetEnabledDeepKeyModelNamesIgnoresDisabledAndOtherChannels(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Channel{}))
	defer DB.Where("name in ?", []string{"deepkey-enabled-test", "deepkey-disabled-test", "other-test"}).Delete(&Channel{})
	deepKeyURL := "https://deepkey.top"
	otherURL := "https://other.example.com"
	require.NoError(t, DB.Create(&[]Channel{
		{Name: "deepkey-enabled-test", Key: "key-1", Status: common.ChannelStatusEnabled, BaseURL: &deepKeyURL, Models: "gpt, image"},
		{Name: "deepkey-disabled-test", Key: "key-2", Status: common.ChannelStatusManuallyDisabled, BaseURL: &deepKeyURL, Models: "disabled"},
		{Name: "other-test", Key: "key-3", Status: common.ChannelStatusEnabled, BaseURL: &otherURL, Models: "other"},
	}).Error)

	models, err := GetEnabledDeepKeyModelNames()
	require.NoError(t, err)
	assert.Equal(t, []string{"gpt", "image"}, models)
}
