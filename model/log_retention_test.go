package model

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteExpiredConsumeLogsKeepsRecentAndNonConsumeRows(t *testing.T) {
	const cutoff int64 = 1000
	const userID = 910001
	const otherUserID = 910002

	require.NoError(t, LOG_DB.Where("user_id IN ?", []int{userID, otherUserID}).Delete(&Log{}).Error)
	t.Cleanup(func() {
		require.NoError(t, LOG_DB.Where("user_id IN ?", []int{userID, otherUserID}).Delete(&Log{}).Error)
	})

	logs := []Log{
		{UserId: userID, Type: LogTypeConsume, CreatedAt: cutoff - 2, RequestId: "expired-consume-1"},
		{UserId: otherUserID, Type: LogTypeConsume, CreatedAt: cutoff - 1, RequestId: "expired-consume-2"},
		{UserId: userID, Type: LogTypeConsume, CreatedAt: cutoff, RequestId: "boundary-consume"},
		{UserId: userID, Type: LogTypeConsume, CreatedAt: cutoff + 1, RequestId: "recent-consume"},
		{UserId: userID, Type: LogTypeTopup, CreatedAt: cutoff - 3, RequestId: "expired-topup"},
	}
	require.NoError(t, LOG_DB.CreateInBatches(logs, 100).Error)

	deleted, err := DeleteExpiredConsumeLogs(context.Background(), cutoff, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	deleted, err = DeleteExpiredConsumeLogs(context.Background(), cutoff, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(1), deleted)

	deleted, err = DeleteExpiredConsumeLogs(context.Background(), cutoff, 1)
	require.NoError(t, err)
	assert.Zero(t, deleted)

	var retained []Log
	require.NoError(t, LOG_DB.Where("user_id IN ?", []int{userID, otherUserID}).Order("created_at asc").Find(&retained).Error)
	require.Len(t, retained, 3)
	assert.Equal(t, []string{"expired-topup", "boundary-consume", "recent-consume"}, []string{
		retained[0].RequestId,
		retained[1].RequestId,
		retained[2].RequestId,
	})
}

func TestGetUserLogsCapsVisibleHistoryByUserAcrossTokens(t *testing.T) {
	const userID = 910003
	require.NoError(t, LOG_DB.Where("user_id = ?", userID).Delete(&Log{}).Error)
	t.Cleanup(func() {
		require.NoError(t, LOG_DB.Where("user_id = ?", userID).Delete(&Log{}).Error)
	})

	logs := make([]Log, 0, UserLogDisplayLimit+1)
	for i := 1; i <= UserLogDisplayLimit+1; i++ {
		logs = append(logs, Log{
			UserId:    userID,
			TokenId:   i%2 + 1,
			CreatedAt: int64(i),
			RequestId: fmt.Sprintf("visible-%03d", i),
		})
	}
	require.NoError(t, LOG_DB.CreateInBatches(logs, 100).Error)

	logsPage, total, err := GetUserLogs(userID, LogTypeUnknown, 0, 0, "", "", 490, 20, "", "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(UserLogDisplayLimit), total)
	require.Len(t, logsPage, 10)
	assert.NotEqual(t, logsPage[0].TokenId, logsPage[1].TokenId)

	beyondLimit, total, err := GetUserLogs(userID, LogTypeUnknown, 0, 0, "", "", UserLogDisplayLimit, 10, "", "", "")
	require.NoError(t, err)
	assert.Equal(t, int64(UserLogDisplayLimit), total)
	assert.Empty(t, beyondLimit)
}
