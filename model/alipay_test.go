package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func insertAlipayOrderForTest(t *testing.T, userID int, tradeNo string, amountFen int64) {
	t.Helper()
	insertUserForPaymentGuardTest(t, userID, 0)
	topUp := &TopUp{
		UserId:          userID,
		Amount:          2,
		Money:           float64(amountFen) / 100,
		TradeNo:         tradeNo,
		PaymentMethod:   PaymentMethodAlipayPrecreate,
		PaymentProvider: PaymentProviderAlipayPrecreate,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	order := &AlipayOrder{
		UserId:          userID,
		ClientRequestId: "request_alipay_123456",
		OutTradeNo:      tradeNo,
		AmountFen:       amountFen,
		CreditQuota:     common.QuotaFromDecimal(decimal.NewFromInt(2).Mul(decimal.NewFromFloat(common.QuotaPerUnit))),
		Currency:        "CNY",
		Status:          AlipayOrderStatusPending,
		ExpiresAt:       time.Now().Add(15 * time.Minute).Unix(),
		CreatedAt:       time.Now().Unix(),
		UpdatedAt:       time.Now().Unix(),
	}
	require.NoError(t, CreateAlipayTopUp(topUp, order))
}

func TestCompleteAlipayTopUp_IsIdempotentAcrossNotifications(t *testing.T) {
	truncateTables(t)
	insertAlipayOrderForTest(t, 701, "alipay-idempotent", 199)

	completion := AlipayCompletion{
		EventID:     "notify-idempotent-1",
		OutTradeNo:  "alipay-idempotent",
		TradeNo:     "trade-idempotent",
		AmountFen:   199,
		Currency:    "CNY",
		SuccessTime: time.Now(),
		BodyDigest:  "digest-1",
	}
	credited, err := CompleteAlipayTopUp(completion)
	require.NoError(t, err)
	assert.True(t, credited)

	expectedQuota, clamp := common.QuotaFromDecimalChecked(decimal.NewFromInt(2).Mul(decimal.NewFromFloat(common.QuotaPerUnit)))
	require.Nil(t, clamp)
	assert.Equal(t, expectedQuota, getUserQuotaForPaymentGuardTest(t, 701))

	credited, err = CompleteAlipayTopUp(completion)
	require.NoError(t, err)
	assert.False(t, credited)
	assert.Equal(t, expectedQuota, getUserQuotaForPaymentGuardTest(t, 701))

	completion.EventID = "notify-idempotent-2"
	completion.BodyDigest = "digest-2"
	credited, err = CompleteAlipayTopUp(completion)
	require.NoError(t, err)
	assert.False(t, credited)
	assert.Equal(t, expectedQuota, getUserQuotaForPaymentGuardTest(t, 701))
}

func TestCompleteAlipayTopUp_RejectsAmountMismatch(t *testing.T) {
	truncateTables(t)
	insertAlipayOrderForTest(t, 702, "alipay-amount-mismatch", 299)

	credited, err := CompleteAlipayTopUp(AlipayCompletion{
		EventID:     "notify-amount-mismatch",
		OutTradeNo:  "alipay-amount-mismatch",
		TradeNo:     "trade-amount-mismatch",
		AmountFen:   298,
		Currency:    "CNY",
		SuccessTime: time.Now(),
		BodyDigest:  "digest-mismatch",
	})
	require.Error(t, err)
	assert.False(t, credited)
	assert.Equal(t, 0, getUserQuotaForPaymentGuardTest(t, 702))
	assert.Equal(t, common.TopUpStatusPending, getTopUpStatusForPaymentGuardTest(t, "alipay-amount-mismatch"))

	var notificationCount int64
	require.NoError(t, DB.Model(&AlipayNotification{}).Count(&notificationCount).Error)
	assert.Zero(t, notificationCount)
}
