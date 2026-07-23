package model

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestPaymentTransactionDatabaseMatrix(t *testing.T) {
	dialect := strings.ToLower(strings.TrimSpace(os.Getenv("PAYMENT_TEST_DIALECT")))
	dsn := strings.TrimSpace(os.Getenv("PAYMENT_TEST_DSN"))
	if dialect == "" || dsn == "" {
		t.Skip("PAYMENT_TEST_DIALECT and PAYMENT_TEST_DSN are required")
	}

	var dialector gorm.Dialector
	var databaseType common.DatabaseType
	switch dialect {
	case "sqlite":
		dialector = sqlite.Open(dsn)
		databaseType = common.DatabaseTypeSQLite
	case "mysql":
		dialector = mysql.Open(dsn)
		databaseType = common.DatabaseTypeMySQL
	case "postgres":
		dialector = postgres.Open(dsn)
		databaseType = common.DatabaseTypePostgreSQL
	default:
		t.Fatalf("unsupported PAYMENT_TEST_DIALECT %q", dialect)
	}

	db, err := gorm.Open(dialector, &gorm.Config{})
	require.NoError(t, err)
	previousDB := DB
	previousLogDB := LOG_DB
	previousMainType := common.MainDatabaseType()
	previousLogType := common.LogDatabaseType()
	DB = db
	LOG_DB = db
	common.SetDatabaseTypes(databaseType, databaseType)
	initCol()
	t.Cleanup(func() {
		if sqlDB, sqlErr := db.DB(); sqlErr == nil {
			_ = sqlDB.Close()
		}
		DB = previousDB
		LOG_DB = previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		initCol()
	})

	if databaseType == common.DatabaseTypeSQLite {
		sqlDB, sqlErr := db.DB()
		require.NoError(t, sqlErr)
		sqlDB.SetMaxOpenConns(1)
	}
	require.NoError(t, db.AutoMigrate(
		&User{},
		&Log{},
		&TopUp{},
		&WechatPayOrder{},
		&WechatPayNotification{},
		&AlipayOrder{},
		&AlipayNotification{},
	))

	suffix := fmt.Sprintf("%s-%d", dialect, time.Now().Unix()%100000000)
	inviter := &User{Username: "payment-matrix-inviter-" + suffix, Status: common.UserStatusEnabled, AffCode: "m-" + suffix}
	require.NoError(t, db.Create(inviter).Error)
	invitee := &User{Username: "payment-matrix-invitee-" + suffix, Status: common.UserStatusEnabled, AffCode: "i-" + suffix, InviterId: inviter.Id}
	require.NoError(t, db.Create(invitee).Error)
	tradeNo := "wechat-matrix-" + suffix
	topUp := &TopUp{
		UserId:          invitee.Id,
		Amount:          50,
		Money:           50,
		TradeNo:         tradeNo,
		PaymentMethod:   PaymentMethodWechatNative,
		PaymentProvider: PaymentProviderWechatNative,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	order := &WechatPayOrder{
		UserId:          invitee.Id,
		ClientRequestId: "matrix-request-" + suffix,
		OutTradeNo:      tradeNo,
		AmountFen:       5000,
		CreditQuota:     5000,
		Currency:        "CNY",
		Status:          WechatPayOrderStatusPending,
		ExpiresAt:       time.Now().Add(time.Minute).Unix(),
		CreatedAt:       time.Now().Unix(),
		UpdatedAt:       time.Now().Unix(),
	}
	require.NoError(t, CreateWechatPayTopUp(topUp, order))

	var waitGroup sync.WaitGroup
	creditedResults := make(chan bool, 2)
	errorResults := make(chan error, 2)
	for index := 0; index < 2; index++ {
		waitGroup.Add(1)
		go func(eventIndex int) {
			defer waitGroup.Done()
			credited, completionErr := CompleteWechatPayTopUp(WechatPayCompletion{
				EventID:       fmt.Sprintf("matrix-event-%s-%d", suffix, eventIndex),
				OutTradeNo:    tradeNo,
				TransactionID: "matrix-transaction-" + suffix,
				AmountFen:     order.AmountFen,
				Currency:      order.Currency,
				SuccessTime:   time.Now(),
				BodyDigest:    fmt.Sprintf("matrix-digest-%d", eventIndex),
			})
			creditedResults <- credited
			errorResults <- completionErr
		}(index)
	}
	waitGroup.Wait()
	close(creditedResults)
	close(errorResults)
	creditedCount := 0
	for credited := range creditedResults {
		if credited {
			creditedCount++
		}
	}
	for completionErr := range errorResults {
		require.NoError(t, completionErr)
	}
	assert.Equal(t, 1, creditedCount)

	var storedInvitee User
	require.NoError(t, db.Where("id = ?", invitee.Id).First(&storedInvitee).Error)
	assert.Equal(t, order.CreditQuota, storedInvitee.Quota)
	var storedInviter User
	require.NoError(t, db.Where("id = ?", inviter.Id).First(&storedInviter).Error)
	assert.Equal(t, 150, storedInviter.AffQuota)
	assert.Equal(t, 150, storedInviter.AffHistoryQuota)
	var storedOrder WechatPayOrder
	require.NoError(t, db.Where("out_trade_no = ?", tradeNo).First(&storedOrder).Error)
	assert.Equal(t, WechatPayOrderStatusCredited, storedOrder.Status)
	var storedTopUp TopUp
	require.NoError(t, db.Where("trade_no = ?", tradeNo).First(&storedTopUp).Error)
	assert.Equal(t, common.TopUpStatusSuccess, storedTopUp.Status)

	overflowUser := &User{Username: "payment-matrix-overflow-" + suffix, Status: common.UserStatusEnabled, AffCode: "o-" + suffix, Quota: common.MaxQuota - 1}
	require.NoError(t, db.Create(overflowUser).Error)
	overflowTradeNo := "alipay-overflow-" + suffix
	overflowTopUp := &TopUp{
		UserId:          overflowUser.Id,
		Amount:          2,
		Money:           1,
		TradeNo:         overflowTradeNo,
		PaymentMethod:   PaymentMethodAlipayPrecreate,
		PaymentProvider: PaymentProviderAlipayPrecreate,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	overflowOrder := &AlipayOrder{
		UserId:          overflowUser.Id,
		ClientRequestId: "matrix-overflow-request-" + suffix,
		OutTradeNo:      overflowTradeNo,
		AmountFen:       100,
		CreditQuota:     2,
		Currency:        "CNY",
		Status:          AlipayOrderStatusPending,
		ExpiresAt:       time.Now().Add(time.Minute).Unix(),
		CreatedAt:       time.Now().Unix(),
		UpdatedAt:       time.Now().Unix(),
	}
	require.NoError(t, CreateAlipayTopUp(overflowTopUp, overflowOrder))
	credited, err := CompleteAlipayTopUp(AlipayCompletion{
		EventID:     "matrix-overflow-event-" + suffix,
		OutTradeNo:  overflowTradeNo,
		TradeNo:     "matrix-alipay-transaction-" + suffix,
		AmountFen:   overflowOrder.AmountFen,
		Currency:    overflowOrder.Currency,
		SuccessTime: time.Now(),
		BodyDigest:  "matrix-overflow-digest",
	})
	require.Error(t, err)
	assert.False(t, credited)
	require.NoError(t, db.Where("id = ?", overflowUser.Id).First(&overflowUser).Error)
	assert.Equal(t, common.MaxQuota-1, overflowUser.Quota)
	require.NoError(t, db.Where("out_trade_no = ?", overflowTradeNo).First(&overflowOrder).Error)
	assert.Equal(t, AlipayOrderStatusPending, overflowOrder.Status)
	require.NoError(t, db.Where("trade_no = ?", overflowTradeNo).First(&overflowTopUp).Error)
	assert.Equal(t, common.TopUpStatusPending, overflowTopUp.Status)
}
