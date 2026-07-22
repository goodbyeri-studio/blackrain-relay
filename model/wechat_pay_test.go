package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertWechatPayOrderForTest(t *testing.T, userID int, tradeNo string, amountFen int64) {
	t.Helper()
	insertUserForPaymentGuardTest(t, userID, 0)
	topUp := &TopUp{
		UserId:          userID,
		Amount:          2,
		Money:           float64(amountFen) / 100,
		TradeNo:         tradeNo,
		PaymentMethod:   PaymentMethodWechatNative,
		PaymentProvider: PaymentProviderWechatNative,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	order := &WechatPayOrder{
		UserId:          userID,
		ClientRequestId: "request_1234567890",
		OutTradeNo:      tradeNo,
		AmountFen:       amountFen,
		CreditQuota:     common.QuotaFromDecimal(decimal.NewFromInt(2).Mul(decimal.NewFromFloat(common.QuotaPerUnit))),
		Currency:        "CNY",
		Status:          WechatPayOrderStatusPending,
		ExpiresAt:       time.Now().Add(15 * time.Minute).Unix(),
		CreatedAt:       time.Now().Unix(),
		UpdatedAt:       time.Now().Unix(),
	}
	require.NoError(t, CreateWechatPayTopUp(topUp, order))
}

func TestCompleteWechatPayTopUp_IsIdempotentAcrossNotifications(t *testing.T) {
	truncateTables(t)
	insertWechatPayOrderForTest(t, 601, "wechat-idempotent", 199)

	completion := WechatPayCompletion{
		EventID:       "event-idempotent-1",
		OutTradeNo:    "wechat-idempotent",
		TransactionID: "transaction-idempotent",
		AmountFen:     199,
		Currency:      "CNY",
		SuccessTime:   time.Now(),
		BodyDigest:    "digest-1",
	}
	credited, err := CompleteWechatPayTopUp(completion)
	require.NoError(t, err)
	assert.True(t, credited)

	expectedQuota, clamp := common.QuotaFromDecimalChecked(decimal.NewFromInt(2).Mul(decimal.NewFromFloat(common.QuotaPerUnit)))
	require.Nil(t, clamp)
	assert.Equal(t, expectedQuota, getUserQuotaForPaymentGuardTest(t, 601))

	credited, err = CompleteWechatPayTopUp(completion)
	require.NoError(t, err)
	assert.False(t, credited)
	assert.Equal(t, expectedQuota, getUserQuotaForPaymentGuardTest(t, 601))

	completion.EventID = "event-idempotent-2"
	completion.BodyDigest = "digest-2"
	credited, err = CompleteWechatPayTopUp(completion)
	require.NoError(t, err)
	assert.False(t, credited)
	assert.Equal(t, expectedQuota, getUserQuotaForPaymentGuardTest(t, 601))
}

func TestCompleteWechatPayTopUp_RejectsAmountMismatch(t *testing.T) {
	truncateTables(t)
	insertWechatPayOrderForTest(t, 602, "wechat-amount-mismatch", 299)

	credited, err := CompleteWechatPayTopUp(WechatPayCompletion{
		EventID:       "event-amount-mismatch",
		OutTradeNo:    "wechat-amount-mismatch",
		TransactionID: "transaction-amount-mismatch",
		AmountFen:     298,
		Currency:      "CNY",
		SuccessTime:   time.Now(),
		BodyDigest:    "digest-mismatch",
	})
	require.Error(t, err)
	assert.False(t, credited)
	assert.Equal(t, 0, getUserQuotaForPaymentGuardTest(t, 602))
	assert.Equal(t, common.TopUpStatusPending, getTopUpStatusForPaymentGuardTest(t, "wechat-amount-mismatch"))

	var notificationCount int64
	require.NoError(t, DB.Model(&WechatPayNotification{}).Count(&notificationCount).Error)
	assert.Zero(t, notificationCount)
}
