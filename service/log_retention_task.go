package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/bytedance/gopkg/util/gopool"
)

const (
	logRetentionInterval         = 5 * time.Minute
	consumeLogRetentionBatchSize = 1000
)

var logRetentionTaskOnce sync.Once

func StartLogRetentionTask() {
	logRetentionTaskOnce.Do(func() {
		if !common.IsMasterNode {
			return
		}

		gopool.Go(func() {
			run := func() {
				ctx := context.Background()
				cutoff := time.Now().Add(-time.Duration(model.ConsumeLogRetentionDays) * 24 * time.Hour).Unix()
				var totalDeleted int64
				for {
					deleted, err := model.DeleteExpiredConsumeLogs(ctx, cutoff, consumeLogRetentionBatchSize)
					if err != nil {
						logger.LogWarn(ctx, fmt.Sprintf("consume log retention cleanup failed: %v", err))
						return
					}
					totalDeleted += deleted
					if deleted < consumeLogRetentionBatchSize {
						break
					}
				}
				if totalDeleted > 0 {
					logger.LogInfo(ctx, fmt.Sprintf("consume log retention cleanup removed %d rows", totalDeleted))
				}
			}

			run()
			ticker := time.NewTicker(logRetentionInterval)
			defer ticker.Stop()
			for range ticker.C {
				run()
			}
		})
	})
}
