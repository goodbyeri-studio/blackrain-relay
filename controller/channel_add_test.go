package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAddChannelRefreshesMemoryRoutingCache(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:add_channel_cache?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.User{}, &model.Log{}))
	require.NoError(t, db.Create(&model.User{
		Id:       1,
		Username: "root-test",
		Role:     common.RoleRootUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalRedisEnabled := common.RedisEnabled
	model.DB = db
	model.LOG_DB = db
	common.MemoryCacheEnabled = true
	common.RedisEnabled = false
	t.Cleanup(func() {
		common.MemoryCacheEnabled = false
		model.InitChannelCache()
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.RedisEnabled = originalRedisEnabled
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	body, err := common.Marshal(AddChannelRequest{
		Mode: "single",
		Channel: &model.Channel{
			Type:    1,
			Key:     "upstream-test-key",
			Status:  common.ChannelStatusEnabled,
			Name:    "newly-added-channel",
			Models:  "gpt-cache-test",
			Group:   "newly-added-group",
			BaseURL: common.GetPointer("https://example.com"),
			Weight:  common.GetPointer(uint(1)),
		},
	})
	require.NoError(t, err)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/channel/", bytes.NewReader(body))
	ctx.Request.Header.Set("Content-Type", "application/json")
	ctx.Set("id", 1)
	ctx.Set("username", "root-test")
	ctx.Set("role", common.RoleRootUser)
	AddChannel(ctx)

	var response tokenAPIResponse
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success, response.Message)

	selected, err := model.GetRandomSatisfiedChannel("newly-added-group", "gpt-cache-test", 0, "/v1/chat/completions")
	require.NoError(t, err)
	require.NotNil(t, selected)
	assert.Equal(t, "newly-added-channel", selected.Name)
}
