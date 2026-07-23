package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedDefaultSubscriptionPlansIsIdempotent(t *testing.T) {
	truncateTables(t)

	require.NoError(t, seedDefaultCNYSubscriptionPlans())
	require.NoError(t, seedDefaultCNYSubscriptionPlans())

	var plans []SubscriptionPlan
	require.NoError(t, DB.Order("sort_order desc").Find(&plans).Error)
	require.Len(t, plans, len(defaultCNYSubscriptionPlans))
	assert.Equal(t, "轻量包", plans[0].Title)
	assert.Equal(t, "CNY", plans[0].Currency)
	assert.Equal(t, float64(10), plans[0].PriceAmount)
	assert.Greater(t, plans[0].TotalAmount, int64(0))
}

func TestSeedDefaultSubscriptionPlansPreservesAdminCatalog(t *testing.T) {
	truncateTables(t)

	customPlan := &SubscriptionPlan{
		Title:            "管理员自定义套餐",
		PriceAmount:      88,
		Currency:         "CNY",
		DurationUnit:     SubscriptionDurationMonth,
		DurationValue:    1,
		Enabled:          true,
		QuotaResetPeriod: SubscriptionResetNever,
	}
	require.NoError(t, DB.Create(customPlan).Error)
	require.NoError(t, seedDefaultCNYSubscriptionPlans())

	var plans []SubscriptionPlan
	require.NoError(t, DB.Find(&plans).Error)
	require.Len(t, plans, 1)
	assert.Equal(t, customPlan.Title, plans[0].Title)
}

func TestCreatePendingSubscriptionOrderReservesPurchaseLimit(t *testing.T) {
	truncateTables(t)

	user := &User{Username: "subscription-reservation-user", Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(user).Error)
	plan := &SubscriptionPlan{
		Title:              "限购套餐",
		PriceAmount:        10,
		Currency:           "CNY",
		DurationUnit:       SubscriptionDurationMonth,
		DurationValue:      1,
		Enabled:            true,
		MaxPurchasePerUser: 1,
		QuotaResetPeriod:   SubscriptionResetNever,
	}
	require.NoError(t, DB.Create(plan).Error)

	first := &SubscriptionOrder{
		UserId:          user.Id,
		PlanId:          plan.Id,
		Money:           plan.PriceAmount,
		TradeNo:         "subscription-reservation-first",
		PaymentMethod:   PaymentMethodWechatNative,
		PaymentProvider: PaymentProviderWechatSubscription,
		Status:          common.TopUpStatusPending,
		CreateTime:      time.Now().Unix(),
	}
	require.NoError(t, CreatePendingSubscriptionOrder(first))

	second := &SubscriptionOrder{
		UserId:          user.Id,
		PlanId:          plan.Id,
		Money:           plan.PriceAmount,
		TradeNo:         "subscription-reservation-second",
		PaymentMethod:   PaymentMethodWechatNative,
		PaymentProvider: PaymentProviderWechatSubscription,
		Status:          common.TopUpStatusPending,
		CreateTime:      time.Now().Unix(),
	}
	require.ErrorIs(t, CreatePendingSubscriptionOrder(second), ErrSubscriptionPurchaseLimit)

	require.NoError(t, DB.Model(first).Update("status", common.TopUpStatusFailed).Error)
	require.NoError(t, CreatePendingSubscriptionOrder(second))
}

func TestBalancePurchaseHonorsPendingReservation(t *testing.T) {
	truncateTables(t)

	user := &User{
		Username: "subscription-balance-reservation-user",
		Status:   common.UserStatusEnabled,
		Quota:    common.MaxQuota,
	}
	require.NoError(t, DB.Create(user).Error)
	plan := &SubscriptionPlan{
		Title:              "余额限购套餐",
		PriceAmount:        10,
		Currency:           "CNY",
		DurationUnit:       SubscriptionDurationMonth,
		DurationValue:      1,
		Enabled:            true,
		MaxPurchasePerUser: 1,
		QuotaResetPeriod:   SubscriptionResetNever,
	}
	require.NoError(t, DB.Create(plan).Error)
	require.NoError(t, CreatePendingSubscriptionOrder(&SubscriptionOrder{
		UserId:          user.Id,
		PlanId:          plan.Id,
		Money:           plan.PriceAmount,
		TradeNo:         "subscription-balance-reservation-pending",
		PaymentMethod:   PaymentMethodWechatNative,
		PaymentProvider: PaymentProviderWechatSubscription,
		Status:          common.TopUpStatusPending,
		CreateTime:      time.Now().Unix(),
	}))

	require.ErrorIs(t, PurchaseSubscriptionWithBalance(user.Id, plan.Id), ErrSubscriptionPurchaseLimit)

	var storedUser User
	require.NoError(t, DB.First(&storedUser, user.Id).Error)
	assert.Equal(t, common.MaxQuota, storedUser.Quota)
	assert.Zero(t, countUserSubscriptionsForPaymentGuardTest(t, user.Id))
}
