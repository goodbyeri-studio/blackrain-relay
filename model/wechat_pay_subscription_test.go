package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompleteWechatPaySubscriptionIsIdempotent(t *testing.T) {
	truncateTables(t)

	user := &User{Id: 9201, Username: "wechat-subscription-user", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(user).Error)
	plan := &SubscriptionPlan{
		Title:            "微信支付测试套餐",
		PriceAmount:      10,
		Currency:         "CNY",
		DurationUnit:     SubscriptionDurationMonth,
		DurationValue:    1,
		Enabled:          true,
		TotalAmount:      1000,
		QuotaResetPeriod: SubscriptionResetNever,
	}
	require.NoError(t, DB.Create(plan).Error)
	now := time.Now()
	subscriptionOrder := &SubscriptionOrder{
		UserId:          user.Id,
		PlanId:          plan.Id,
		Money:           plan.PriceAmount,
		TradeNo:         "wechat-subscription-idempotent",
		PaymentMethod:   PaymentMethodWechatNative,
		PaymentProvider: PaymentProviderWechatSubscription,
		Status:          common.TopUpStatusPending,
		CreateTime:      now.Unix(),
	}
	wechatOrder := &SubscriptionWechatPayOrder{
		UserId:          user.Id,
		ClientRequestId: "wechat_subscription_request",
		OutTradeNo:      subscriptionOrder.TradeNo,
		AmountFen:       1000,
		Currency:        "CNY",
		Status:          WechatPayOrderStatusPending,
		ExpiresAt:       now.Add(15 * time.Minute).Unix(),
		CreatedAt:       now.Unix(),
		UpdatedAt:       now.Unix(),
	}
	require.NoError(t, CreateSubscriptionWechatPayOrder(subscriptionOrder, wechatOrder))

	completion := WechatPayCompletion{
		EventID:       "wechat-subscription-event",
		OutTradeNo:    subscriptionOrder.TradeNo,
		TransactionID: "wechat-subscription-transaction",
		AmountFen:     1000,
		Currency:      "CNY",
		SuccessTime:   now,
		BodyDigest:    "digest",
	}
	credited, err := CompleteWechatPaySubscription(completion, "verified callback")
	require.NoError(t, err)
	assert.True(t, credited)
	credited, err = CompleteWechatPaySubscription(completion, "verified callback")
	require.NoError(t, err)
	assert.False(t, credited)

	storedOrder := GetSubscriptionOrderByTradeNo(subscriptionOrder.TradeNo)
	require.NotNil(t, storedOrder)
	assert.Equal(t, common.TopUpStatusSuccess, storedOrder.Status)
	storedWechatOrder, err := GetSubscriptionWechatPayOrderByTradeNo(subscriptionOrder.TradeNo)
	require.NoError(t, err)
	assert.Equal(t, WechatPayOrderStatusCredited, storedWechatOrder.Status)
	assert.Equal(t, int64(1), countUserSubscriptionsForPaymentGuardTest(t, user.Id))

	var topUp TopUp
	require.NoError(t, DB.Where("trade_no = ?", subscriptionOrder.TradeNo).First(&topUp).Error)
	assert.Equal(t, common.TopUpStatusSuccess, topUp.Status)
	assert.Equal(t, PaymentMethodWechatNative, topUp.PaymentMethod)
}

func TestCompleteWechatPaySubscriptionRejectsAmountMismatch(t *testing.T) {
	truncateTables(t)

	user := &User{Id: 9202, Username: "wechat-subscription-mismatch", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(user).Error)
	plan := insertSubscriptionPlanForPaymentGuardTest(t, 9202)
	subscriptionOrder := &SubscriptionOrder{
		UserId:          user.Id,
		PlanId:          plan.Id,
		Money:           10,
		TradeNo:         "wechat-subscription-mismatch",
		PaymentMethod:   PaymentMethodWechatNative,
		PaymentProvider: PaymentProviderWechatSubscription,
		Status:          common.TopUpStatusPending,
		CreateTime:      time.Now().Unix(),
	}
	wechatOrder := &SubscriptionWechatPayOrder{
		UserId:          user.Id,
		ClientRequestId: "wechat_mismatch_request",
		OutTradeNo:      subscriptionOrder.TradeNo,
		AmountFen:       1000,
		Currency:        "CNY",
		Status:          WechatPayOrderStatusPending,
	}
	require.NoError(t, CreateSubscriptionWechatPayOrder(subscriptionOrder, wechatOrder))

	_, err := CompleteWechatPaySubscription(WechatPayCompletion{
		EventID:       "wechat-mismatch-event",
		OutTradeNo:    subscriptionOrder.TradeNo,
		TransactionID: "wechat-mismatch-transaction",
		AmountFen:     999,
		Currency:      "CNY",
		SuccessTime:   time.Now(),
	}, "verified callback")
	require.Error(t, err)
	storedOrder := GetSubscriptionOrderByTradeNo(subscriptionOrder.TradeNo)
	require.NotNil(t, storedOrder)
	assert.Equal(t, common.TopUpStatusPending, storedOrder.Status)
	assert.Zero(t, countUserSubscriptionsForPaymentGuardTest(t, user.Id))
}

func TestCreateSubscriptionWechatPayOrderRejectsOwnershipMismatch(t *testing.T) {
	truncateTables(t)

	subscriptionOrder := &SubscriptionOrder{
		UserId:  9301,
		TradeNo: "wechat-subscription-owner",
		Status:  common.TopUpStatusPending,
	}
	wechatOrder := &SubscriptionWechatPayOrder{
		UserId:     9302,
		OutTradeNo: subscriptionOrder.TradeNo,
		AmountFen:  1000,
		Currency:   "CNY",
		Status:     WechatPayOrderStatusPending,
	}

	require.Error(t, CreateSubscriptionWechatPayOrder(subscriptionOrder, wechatOrder))
	var orderCount int64
	require.NoError(t, DB.Model(&SubscriptionOrder{}).Count(&orderCount).Error)
	assert.Zero(t, orderCount)
}
