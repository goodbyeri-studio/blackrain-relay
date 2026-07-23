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

	token := &Token{UserId: 1, Key: "sk-deepkey-migration-token", Group: "deepkey"}
	require.NoError(t, DB.Create(token).Error)
	t.Cleanup(func() { DB.Unscoped().Where("key = ?", token.Key).Delete(&Token{}) })
	beforeBlocked, err := GetDeepKeyPricingMigrationSnapshot()
	require.NoError(t, err)
	blockedHash, err := beforeBlocked.Hash()
	require.NoError(t, err)
	err = ApplyDeepKeyPricingMigration(blockedHash, map[string]float64{"gpt": 88}, desiredPrice, map[string]float64{}, "blocked")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "deepkey")
	unchanged, err = GetDeepKeyPricingMigrationSnapshot()
	require.NoError(t, err)
	assert.Equal(t, desiredRatio, unchanged.ModelRatio)
	assert.Equal(t, desiredGroups, unchanged.GroupRatio)
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

func TestIsDeepKeyBaseURLMatchesOnlyConfiguredHost(t *testing.T) {
	assert.True(t, IsDeepKeyBaseURL("https://deepkey.top/v1"))
	assert.True(t, IsDeepKeyBaseURL("https://DEEPKEY.TOP"))
	assert.False(t, IsDeepKeyBaseURL("https://api.deepkey.top"))
	assert.False(t, IsDeepKeyBaseURL("https://deepkey.top.example.com"))
	assert.False(t, IsDeepKeyBaseURL("not-a-url"))
}

func TestGetEnabledDeepKeyChannelGroupsRequiresIsolatedSharedKey(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Channel{}))
	names := []string{"deepkey-group-one", "deepkey-group-two", "deepkey-group-disabled", "other-group"}
	t.Cleanup(func() { DB.Where("name in ?", names).Delete(&Channel{}) })
	deepKeyURL := "https://deepkey.top"
	otherURL := "https://other.example.com"
	require.NoError(t, DB.Create(&[]Channel{
		{Name: names[0], Key: "shared-key", Status: common.ChannelStatusEnabled, BaseURL: &deepKeyURL, Group: "alpha,beta"},
		{Name: names[1], Key: "shared-key", Status: common.ChannelStatusEnabled, BaseURL: &deepKeyURL, Group: "beta"},
		{Name: names[2], Key: "disabled-key", Status: common.ChannelStatusManuallyDisabled, BaseURL: &deepKeyURL, Group: "disabled"},
		{Name: names[3], Key: "other-key", Status: common.ChannelStatusEnabled, BaseURL: &otherURL, Group: "other"},
	}).Error)

	groups, err := GetEnabledDeepKeyChannelGroups()
	require.NoError(t, err)
	assert.Equal(t, map[string]struct{}{"alpha": {}, "beta": {}}, groups)
}

func TestGetEnabledDeepKeyChannelGroupsRejectsRoutingAmbiguity(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Channel{}))
	deepKeyURL := "https://deepkey.top"
	otherURL := "https://other.example.com"

	t.Run("duplicate entries in one DeepKey channel", func(t *testing.T) {
		name := "deepkey-duplicate-key"
		t.Cleanup(func() { DB.Where("name = ?", name).Delete(&Channel{}) })
		require.NoError(t, DB.Create(&Channel{
			Name: name, Key: "key-one\nkey-one", Status: common.ChannelStatusEnabled, BaseURL: &deepKeyURL, Group: "duplicate-key",
		}).Error)

		_, err := GetEnabledDeepKeyChannelGroups()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exactly one upstream key")
	})

	t.Run("different DeepKey keys in one group", func(t *testing.T) {
		names := []string{"deepkey-conflict-one", "deepkey-conflict-two"}
		t.Cleanup(func() { DB.Where("name in ?", names).Delete(&Channel{}) })
		require.NoError(t, DB.Create(&[]Channel{
			{Name: names[0], Key: "key-one", Status: common.ChannelStatusEnabled, BaseURL: &deepKeyURL, Group: "conflict-key"},
			{Name: names[1], Key: "key-two", Status: common.ChannelStatusEnabled, BaseURL: &deepKeyURL, Group: "conflict-key"},
		}).Error)

		_, err := GetEnabledDeepKeyChannelGroups()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "multiple upstream key configurations")
	})

	t.Run("DeepKey and non-DeepKey channels share a group", func(t *testing.T) {
		names := []string{"deepkey-host-conflict", "other-host-conflict"}
		t.Cleanup(func() { DB.Where("name in ?", names).Delete(&Channel{}) })
		require.NoError(t, DB.Create(&[]Channel{
			{Name: names[0], Key: "shared-key", Status: common.ChannelStatusEnabled, BaseURL: &deepKeyURL, Group: "conflict-host"},
			{Name: names[1], Key: "other-key", Status: common.ChannelStatusEnabled, BaseURL: &otherURL, Group: "conflict-host"},
		}).Error)

		_, err := GetEnabledDeepKeyChannelGroups()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "non-DeepKey channel")
	})
}

func TestValidateDeepKeyChannelGroupIsolationChecksCandidate(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Channel{}))
	deepKeyURL := "https://deepkey.top"
	existing := Channel{Name: "deepkey-candidate-existing", Key: "shared-key", Status: common.ChannelStatusEnabled, BaseURL: &deepKeyURL, Group: "candidate-group"}
	require.NoError(t, DB.Create(&existing).Error)
	t.Cleanup(func() { DB.Where("name = ?", existing.Name).Delete(&Channel{}) })

	compatible := Channel{Key: "shared-key", Status: common.ChannelStatusEnabled, BaseURL: &deepKeyURL, Group: "candidate-group"}
	require.NoError(t, ValidateDeepKeyChannelGroupIsolation(&compatible))

	conflicting := Channel{Key: "different-key", Status: common.ChannelStatusEnabled, BaseURL: &deepKeyURL, Group: "candidate-group"}
	err := ValidateDeepKeyChannelGroupIsolation(&conflicting)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple upstream key configurations")
}

func TestUpdateChannelStatusRejectsInvalidDeepKeyEnable(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Channel{}))
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = previousMemoryCacheEnabled })

	deepKeyURL := "https://deepkey.top"
	names := []string{"deepkey-enable-existing", "deepkey-enable-conflict"}
	t.Cleanup(func() { DB.Where("name in ?", names).Delete(&Channel{}) })
	existing := Channel{
		Name: names[0], Key: "key-one", Status: common.ChannelStatusEnabled,
		BaseURL: &deepKeyURL, Group: "enable-conflict",
	}
	conflicting := Channel{
		Name: names[1], Key: "key-two", Status: common.ChannelStatusManuallyDisabled,
		BaseURL: &deepKeyURL, Group: "enable-conflict",
	}
	require.NoError(t, DB.Create(&existing).Error)
	require.NoError(t, DB.Create(&conflicting).Error)

	assert.False(t, UpdateChannelStatus(conflicting.Id, "", common.ChannelStatusEnabled, "test"))
	require.NoError(t, DB.First(&conflicting, conflicting.Id).Error)
	assert.Equal(t, common.ChannelStatusManuallyDisabled, conflicting.Status)
}

func TestEnableChannelByTagRejectsInvalidDeepKeyGroup(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Channel{}))
	deepKeyURL := "https://deepkey.top"
	tag := "deepkey-enable-tag-conflict"
	names := []string{"deepkey-tag-existing", "deepkey-tag-conflict"}
	t.Cleanup(func() { DB.Where("name in ?", names).Delete(&Channel{}) })
	existing := Channel{
		Name: names[0], Key: "key-one", Status: common.ChannelStatusEnabled,
		BaseURL: &deepKeyURL, Group: "tag-conflict",
	}
	conflicting := Channel{
		Name: names[1], Key: "key-two", Status: common.ChannelStatusManuallyDisabled,
		BaseURL: &deepKeyURL, Group: "tag-conflict", Tag: &tag,
	}
	require.NoError(t, DB.Create(&existing).Error)
	require.NoError(t, DB.Create(&conflicting).Error)

	err := EnableChannelByTag(tag)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple upstream key configurations")
	require.NoError(t, DB.First(&conflicting, conflicting.Id).Error)
	assert.Equal(t, common.ChannelStatusManuallyDisabled, conflicting.Status)
}

func TestEditChannelByTagRejectsInvalidDeepKeyGroup(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Channel{}))
	deepKeyURL := "https://deepkey.top"
	tag := "deepkey-edit-tag-conflict"
	names := []string{"deepkey-edit-tag-existing", "deepkey-edit-tag-conflict"}
	t.Cleanup(func() { DB.Where("name in ?", names).Delete(&Channel{}) })
	existing := Channel{
		Name: names[0], Key: "key-one", Status: common.ChannelStatusEnabled,
		BaseURL: &deepKeyURL, Group: "edit-tag-existing",
	}
	conflicting := Channel{
		Name: names[1], Key: "key-two", Status: common.ChannelStatusEnabled,
		BaseURL: &deepKeyURL, Group: "edit-tag-original", Tag: &tag,
	}
	require.NoError(t, DB.Create(&existing).Error)
	require.NoError(t, DB.Create(&conflicting).Error)

	newGroup := "edit-tag-existing"
	err := EditChannelByTag(tag, nil, nil, nil, &newGroup, nil, nil, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "multiple upstream key configurations")
	require.NoError(t, DB.First(&conflicting, conflicting.Id).Error)
	assert.Equal(t, "edit-tag-original", conflicting.Group)
}

func TestGetDeepKeyChannelGroupStatusesRedactsKeysAndCountsTokens(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Channel{}, &Token{}))
	deepKeyURL := "https://deepkey.top"
	channelNames := []string{"deepkey-status-enabled", "deepkey-status-disabled", "deepkey-status-duplicate"}
	tokenKeys := []string{"sk-deepkey-status-token"}
	t.Cleanup(func() {
		DB.Where("name in ?", channelNames).Delete(&Channel{})
		DB.Where("key in ?", tokenKeys).Delete(&Token{})
	})
	require.NoError(t, DB.Create(&[]Channel{
		{Name: channelNames[0], Key: "upstream-secret-status", Status: common.ChannelStatusEnabled, BaseURL: &deepKeyURL, Models: "gpt-4o,image-1", Group: "status-group", TestTime: 123, ResponseTime: 42},
		{Name: channelNames[1], Key: "upstream-secret-status", Status: common.ChannelStatusManuallyDisabled, BaseURL: &deepKeyURL, Models: "image-1", Group: "status-group"},
		{Name: channelNames[2], Key: "duplicate-secret\nduplicate-secret", Status: common.ChannelStatusEnabled, BaseURL: &deepKeyURL, Models: "gpt-4o", Group: "duplicate-status-group"},
	}).Error)
	require.NoError(t, DB.Create(&Token{UserId: 1, Key: tokenKeys[0], Group: "status-group"}).Error)

	statuses, err := GetDeepKeyChannelGroupStatuses()
	require.NoError(t, err)
	status, ok := statuses["status-group"]
	require.True(t, ok)
	assert.Equal(t, 2, status.ChannelCount)
	assert.Equal(t, 1, status.EnabledChannelCount)
	assert.Equal(t, 1, status.DisabledChannelCount)
	assert.Equal(t, 2, status.ModelCount)
	assert.Equal(t, int64(1), status.TokenCount)
	assert.NotContains(t, status.KeyFingerprint, "upstream-secret-status")
	assert.Len(t, status.KeyFingerprint, 16)
	assert.True(t, status.KeyConfigurationValid)
	assert.Equal(t, int64(123), status.LastTestTime)
	assert.Equal(t, 42, status.ResponseTime)
	assert.False(t, statuses["duplicate-status-group"].KeyConfigurationValid)
}

func TestCountTokensByGroupsFiltersRequestedGroups(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Token{}))
	tokenKeys := []string{"sk-count-status-a", "sk-count-status-b"}
	t.Cleanup(func() { DB.Where("key in ?", tokenKeys).Delete(&Token{}) })
	require.NoError(t, DB.Create(&[]Token{
		{UserId: 1, Key: tokenKeys[0], Group: "count-a"},
		{UserId: 1, Key: tokenKeys[1], Group: "count-b"},
	}).Error)

	counts, err := CountTokensByGroups([]string{"count-a"})
	require.NoError(t, err)
	assert.Equal(t, int64(1), counts["count-a"])
	assert.NotContains(t, counts, "count-b")
}
