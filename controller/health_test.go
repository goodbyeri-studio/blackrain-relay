package controller

import (
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

func TestGetLiveHealth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)

	GetLiveHealth(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{"status":"ok"}`, recorder.Body.String())
}

func TestGetReadyHealthWhenDependenciesAreAvailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:health_ready?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)

	originalDB := model.DB
	originalRedisEnabled := common.RedisEnabled
	model.DB = db
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = originalDB
		common.RedisEnabled = originalRedisEnabled
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/healthz/ready", nil)
	GetReadyHealth(ctx)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{"status":"ready"}`, recorder.Body.String())
}

func TestGetReadyHealthWhenDatabaseIsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalDB := model.DB
	originalRedisEnabled := common.RedisEnabled
	model.DB = nil
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = originalDB
		common.RedisEnabled = originalRedisEnabled
	})

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/healthz/ready", nil)
	GetReadyHealth(ctx)

	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.JSONEq(t, `{"status":"unavailable"}`, recorder.Body.String())
}
