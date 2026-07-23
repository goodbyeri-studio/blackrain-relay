package model

import (
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCompleteEpayTopUpCreditsAffiliateRewardOnce(t *testing.T) {
	truncateTables(t)
	inviter := &User{
		Id:       811,
		Username: "epay_affiliate_inviter",
		Status:   common.UserStatusEnabled,
		AffCode:  "epay-inviter",
	}
	invitee := &User{
		Id:        812,
		Username:  "epay_affiliate_invitee",
		Status:    common.UserStatusEnabled,
		InviterId: inviter.Id,
		AffCode:   "epay-invitee",
	}
	require.NoError(t, DB.Create(inviter).Error)
	require.NoError(t, DB.Create(invitee).Error)
	require.NoError(t, DB.Create(&TopUp{
		UserId:          invitee.Id,
		Amount:          50,
		Money:           50,
		TradeNo:         "epay-affiliate-reward",
		PaymentMethod:   "alipay",
		PaymentProvider: PaymentProviderEpay,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}).Error)

	topUp, creditedQuota, credited, err := CompleteEpayTopUp("epay-affiliate-reward", "alipay")
	require.NoError(t, err)
	require.NotNil(t, topUp)
	assert.True(t, credited)

	expectedReward := common.QuotaFromDecimal(
		decimal.NewFromInt(int64(creditedQuota)).Mul(decimal.NewFromInt(3)).Div(decimal.NewFromInt(100)),
	)
	var creditedInviter User
	require.NoError(t, DB.Where("id = ?", inviter.Id).First(&creditedInviter).Error)
	assert.Equal(t, expectedReward, creditedInviter.AffQuota)
	assert.Equal(t, expectedReward, creditedInviter.AffHistoryQuota)

	_, _, credited, err = CompleteEpayTopUp("epay-affiliate-reward", "alipay")
	require.NoError(t, err)
	assert.False(t, credited)
	require.NoError(t, DB.Where("id = ?", inviter.Id).First(&creditedInviter).Error)
	assert.Equal(t, expectedReward, creditedInviter.AffQuota)
	assert.Equal(t, expectedReward, creditedInviter.AffHistoryQuota)
}

func TestInsertWithTxPersistsInviterForTopUpRewards(t *testing.T) {
	truncateTables(t)
	inviter := &User{
		Id:       801,
		Username: "oauth_affiliate_inviter",
		Status:   common.UserStatusEnabled,
		AffCode:  "oauth-inviter",
	}
	require.NoError(t, DB.Create(inviter).Error)

	invitee := &User{
		Username: "oauth_affiliate_invitee",
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		return invitee.InsertWithTx(tx, inviter.Id)
	}))

	var savedInvitee User
	require.NoError(t, DB.Where("id = ?", invitee.Id).First(&savedInvitee).Error)
	assert.Equal(t, inviter.Id, savedInvitee.InviterId)
}

func TestCreditAffiliateTopUpRewardKeepsBalanceAndHistoryConsistentAtLimit(t *testing.T) {
	truncateTables(t)
	inviter := &User{
		Id:              821,
		Username:        "limited_affiliate_inviter",
		Status:          common.UserStatusEnabled,
		AffCode:         "limited-inviter",
		AffQuota:        common.MaxQuota - 1,
		AffHistoryQuota: 0,
	}
	invitee := &User{
		Id:        822,
		Username:  "limited_affiliate_invitee",
		Status:    common.UserStatusEnabled,
		InviterId: inviter.Id,
		AffCode:   "limited-invitee",
	}
	require.NoError(t, DB.Create(inviter).Error)
	require.NoError(t, DB.Create(invitee).Error)

	var reward *affiliateTopUpReward
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var err error
		reward, err = creditAffiliateTopUpReward(tx, invitee.Id, 100)
		return err
	}))
	require.NotNil(t, reward)
	assert.Equal(t, 1, reward.Quota)

	var creditedInviter User
	require.NoError(t, DB.Where("id = ?", inviter.Id).First(&creditedInviter).Error)
	assert.Equal(t, common.MaxQuota, creditedInviter.AffQuota)
	assert.Equal(t, 1, creditedInviter.AffHistoryQuota)
}
