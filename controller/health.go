package controller

import (
	"context"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// GetLiveHealth reports process liveness without checking external dependencies.
func GetLiveHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// GetReadyHealth reports whether the process can serve requests that need its dependencies.
func GetReadyHealth(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()

	if model.DB == nil {
		respondHealthUnavailable(c)
		return
	}

	sqlDB, err := model.DB.DB()
	if err != nil || sqlDB.PingContext(ctx) != nil {
		respondHealthUnavailable(c)
		return
	}

	if common.RedisEnabled {
		if common.RDB == nil || common.RDB.Ping(ctx).Err() != nil {
			respondHealthUnavailable(c)
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

func respondHealthUnavailable(c *gin.Context) {
	c.JSON(http.StatusServiceUnavailable, gin.H{"status": "unavailable"})
}
