package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUpdateOptionRejectsRemovingGroupUsedByToken(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Option{}, &Token{}))
	originalRatio := ratio_setting.GroupRatio2JSONString()
	t.Cleanup(func() {
		ratio_setting.UpdateGroupRatioByJSONString(originalRatio)
		DB.Unscoped().Where("key = ?", "sk-group-ratio-protected").Delete(&Token{})
		DB.Where("key = ?", "GroupRatio").Delete(&Option{})
	})
	require.NoError(t, DB.Where("key = ?", "GroupRatio").Delete(&Option{}).Error)
	require.NoError(t, DB.Create(&Option{Key: "GroupRatio", Value: `{"protected":1,"keep":1}`}).Error)
	require.NoError(t, DB.Create(&Token{UserId: 1, Key: "sk-group-ratio-protected", Group: "protected"}).Error)

	err := UpdateOption("GroupRatio", `{"keep":1}`)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "protected")

	var stored Option
	require.NoError(t, DB.First(&stored, "key = ?", "GroupRatio").Error)
	assert.Equal(t, `{"protected":1,"keep":1}`, stored.Value)
}

func TestUpdateOptionsBulkIgnoresSoftDeletedTokensAndRollsBackAllWritesOnRejection(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Option{}, &Token{}))
	originalRatio := ratio_setting.GroupRatio2JSONString()
	common.OptionMapRWMutex.RLock()
	optionMapWasNil := common.OptionMap == nil
	originalOptionMap := make(map[string]string, len(common.OptionMap))
	for key, value := range common.OptionMap {
		originalOptionMap[key] = value
	}
	common.OptionMapRWMutex.RUnlock()
	if optionMapWasNil {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = make(map[string]string)
		common.OptionMapRWMutex.Unlock()
	}
	t.Cleanup(func() {
		ratio_setting.UpdateGroupRatioByJSONString(originalRatio)
		common.OptionMapRWMutex.Lock()
		if optionMapWasNil {
			common.OptionMap = nil
		} else {
			common.OptionMap = originalOptionMap
		}
		common.OptionMapRWMutex.Unlock()
		DB.Unscoped().Where("key in ?", []string{"sk-group-ratio-soft-deleted", "sk-group-ratio-active"}).Delete(&Token{})
		DB.Where("key in ?", []string{"GroupRatio", "Notice"}).Delete(&Option{})
	})
	require.NoError(t, DB.Where("key in ?", []string{"GroupRatio", "Notice"}).Delete(&Option{}).Error)
	require.NoError(t, DB.Create(&Option{Key: "GroupRatio", Value: `{"soft-deleted":1,"active":1}`}).Error)
	softDeleted := &Token{UserId: 1, Key: "sk-group-ratio-soft-deleted", Group: "soft-deleted"}
	require.NoError(t, DB.Create(softDeleted).Error)
	require.NoError(t, DB.Delete(softDeleted).Error)

	require.NoError(t, UpdateOptionsBulk(map[string]string{
		"GroupRatio": `{"active":1}`,
		"Notice":     "should persist",
	}))
	var stored Option
	require.NoError(t, DB.First(&stored, "key = ?", "GroupRatio").Error)
	assert.Equal(t, `{"active":1}`, stored.Value)

	active := &Token{UserId: 1, Key: "sk-group-ratio-active", Group: "active"}
	require.NoError(t, DB.Create(active).Error)
	err := UpdateOptionsBulk(map[string]string{
		"GroupRatio": `{"other":1}`,
		"Notice":     "must roll back",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "active")
	assert.ErrorIs(t, DB.First(&Option{}, "key = ? and value = ?", "Notice", "must roll back").Error, gorm.ErrRecordNotFound)
}

func TestTokenWritesValidateGroupAgainstDatabaseOption(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Option{}, &Token{}))
	t.Cleanup(func() {
		DB.Unscoped().Where("key in ?", []string{"sk-token-missing-group", "sk-token-existing-group", "sk-token-auto"}).Delete(&Token{})
		DB.Where("key = ?", "GroupRatio").Delete(&Option{})
	})
	require.NoError(t, DB.Where("key = ?", "GroupRatio").Delete(&Option{}).Error)
	require.NoError(t, DB.Create(&Option{Key: "GroupRatio", Value: `{"existing":1}`}).Error)

	err := (&Token{UserId: 1, Key: "sk-token-missing-group", Group: "missing"}).Insert()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing")

	token := &Token{UserId: 1, Key: "sk-token-existing-group", Group: "existing"}
	require.NoError(t, DB.Create(token).Error)
	token.Group = "missing"
	err = token.UpdateWithGroupValidation()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing")
	var stored Token
	require.NoError(t, DB.First(&stored, "key = ?", token.Key).Error)
	assert.Equal(t, "existing", stored.Group)

	assert.NoError(t, (&Token{UserId: 1, Key: "sk-token-auto", Group: "auto"}).Insert())
}
