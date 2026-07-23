package model

import (
	"errors"
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	PaymentMethodWechatNative         = "wechat_native"
	PaymentProviderWechatNative       = "wechat_native"
	PaymentProviderWechatSubscription = "wechat_native_subscription"

	WechatPayOrderStatusPending  = "pending"
	WechatPayOrderStatusCredited = "credited"
	WechatPayOrderStatusFailed   = "failed"
	WechatPayOrderStatusClosed   = "closed"
)

type WechatPayOrder struct {
	Id                  int     `json:"id"`
	TopUpId             int     `json:"topup_id" gorm:"uniqueIndex"`
	UserId              int     `json:"user_id" gorm:"index;uniqueIndex:idx_wechat_user_request"`
	ClientRequestId     string  `json:"client_request_id" gorm:"type:varchar(64);uniqueIndex:idx_wechat_user_request"`
	OutTradeNo          string  `json:"out_trade_no" gorm:"type:varchar(64);uniqueIndex"`
	AmountFen           int64   `json:"amount_fen"`
	CreditQuota         int     `json:"credit_quota"`
	Currency            string  `json:"currency" gorm:"type:varchar(8)"`
	Status              string  `json:"status" gorm:"type:varchar(24);index"`
	CodeUrl             string  `json:"-" gorm:"type:text"`
	WechatTransactionId *string `json:"-" gorm:"type:varchar(64);uniqueIndex"`
	ExpiresAt           int64   `json:"expires_at"`
	SuccessTime         int64   `json:"success_time"`
	CreatedAt           int64   `json:"created_at"`
	UpdatedAt           int64   `json:"updated_at"`
	LastCheckedAt       int64   `json:"last_checked_at"`
}

// SubscriptionWechatPayOrder stores the Native QR order that settles a
// subscription order. It is deliberately separate from WechatPayOrder so a
// wallet top-up can never be mistaken for a plan purchase during callbacks.
type SubscriptionWechatPayOrder struct {
	Id                  int     `json:"id"`
	SubscriptionOrderId int     `json:"subscription_order_id" gorm:"uniqueIndex"`
	UserId              int     `json:"user_id" gorm:"index;uniqueIndex:idx_subscription_wechat_user_request"`
	ClientRequestId     string  `json:"client_request_id" gorm:"type:varchar(64);uniqueIndex:idx_subscription_wechat_user_request"`
	OutTradeNo          string  `json:"out_trade_no" gorm:"type:varchar(64);uniqueIndex"`
	AmountFen           int64   `json:"amount_fen"`
	Currency            string  `json:"currency" gorm:"type:varchar(8)"`
	Status              string  `json:"status" gorm:"type:varchar(24);index"`
	CodeUrl             string  `json:"-" gorm:"type:text"`
	WechatTransactionId *string `json:"-" gorm:"type:varchar(64);uniqueIndex"`
	ExpiresAt           int64   `json:"expires_at"`
	SuccessTime         int64   `json:"success_time"`
	CreatedAt           int64   `json:"created_at"`
	UpdatedAt           int64   `json:"updated_at"`
	LastCheckedAt       int64   `json:"last_checked_at"`
}

type WechatPayNotification struct {
	Id               int    `json:"id"`
	EventId          string `json:"event_id" gorm:"type:varchar(64);uniqueIndex"`
	OutTradeNo       string `json:"out_trade_no" gorm:"type:varchar(64);index"`
	BodyDigest       string `json:"body_digest" gorm:"type:varchar(64)"`
	ProcessingStatus string `json:"processing_status" gorm:"type:varchar(24)"`
	ReceivedAt       int64  `json:"received_at"`
	ProcessedAt      int64  `json:"processed_at"`
}

type WechatPayCompletion struct {
	EventID       string
	OutTradeNo    string
	TransactionID string
	AmountFen     int64
	Currency      string
	SuccessTime   time.Time
	BodyDigest    string
}

func CreateWechatPayTopUp(topUp *TopUp, order *WechatPayOrder) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(topUp).Error; err != nil {
			return err
		}
		order.TopUpId = topUp.Id
		return tx.Create(order).Error
	})
}

func GetWechatPayOrderByClientRequest(userID int, clientRequestID string) (*WechatPayOrder, error) {
	var order WechatPayOrder
	if err := DB.Where("user_id = ? AND client_request_id = ?", userID, clientRequestID).First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func GetWechatPayOrderForUser(userID int, tradeNo string) (*WechatPayOrder, error) {
	var order WechatPayOrder
	if err := DB.Where("user_id = ? AND out_trade_no = ?", userID, tradeNo).First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func GetSubscriptionWechatPayOrderByClientRequest(userID int, clientRequestID string) (*SubscriptionWechatPayOrder, error) {
	var order SubscriptionWechatPayOrder
	if err := DB.Where("user_id = ? AND client_request_id = ?", userID, clientRequestID).First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func GetSubscriptionWechatPayOrderByTradeNo(tradeNo string) (*SubscriptionWechatPayOrder, error) {
	var order SubscriptionWechatPayOrder
	if err := DB.Where("out_trade_no = ?", tradeNo).First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func GetSubscriptionWechatPayOrderForUser(userID int, tradeNo string) (*SubscriptionWechatPayOrder, error) {
	var order SubscriptionWechatPayOrder
	if err := DB.Where("user_id = ? AND out_trade_no = ?", userID, tradeNo).First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func CreateSubscriptionWechatPayOrder(subscriptionOrder *SubscriptionOrder, order *SubscriptionWechatPayOrder) error {
	if subscriptionOrder == nil || order == nil {
		return errors.New("invalid subscription wechat order")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(subscriptionOrder).Error; err != nil {
			return err
		}
		order.SubscriptionOrderId = subscriptionOrder.Id
		return tx.Create(order).Error
	})
}

func UpdateSubscriptionWechatPayPrepayResult(tradeNo string, codeURL string) error {
	return DB.Model(&SubscriptionWechatPayOrder{}).
		Where("out_trade_no = ? AND status = ?", tradeNo, WechatPayOrderStatusPending).
		Updates(map[string]interface{}{"code_url": codeURL, "updated_at": common.GetTimestamp()}).Error
}

func MarkSubscriptionWechatPayOrderFailed(tradeNo string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&SubscriptionWechatPayOrder{}).
			Where("out_trade_no = ? AND status = ?", tradeNo, WechatPayOrderStatusPending).
			Updates(map[string]interface{}{"status": WechatPayOrderStatusFailed, "updated_at": common.GetTimestamp()}).Error; err != nil {
			return err
		}
		return tx.Model(&SubscriptionOrder{}).
			Where("trade_no = ? AND payment_provider = ? AND status = ?", tradeNo, PaymentProviderWechatSubscription, common.TopUpStatusPending).
			Updates(map[string]interface{}{"status": common.TopUpStatusFailed, "complete_time": common.GetTimestamp()}).Error
	})
}

func ClaimSubscriptionWechatPayOrderCheck(tradeNo string, minimumInterval time.Duration) (bool, error) {
	now := common.GetTimestamp()
	cutoff := now - int64(minimumInterval/time.Second)
	result := DB.Model(&SubscriptionWechatPayOrder{}).
		Where("out_trade_no = ? AND status = ? AND last_checked_at <= ?", tradeNo, WechatPayOrderStatusPending, cutoff).
		Update("last_checked_at", now)
	return result.RowsAffected == 1, result.Error
}

func CloseSubscriptionWechatPayOrder(tradeNo string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&SubscriptionWechatPayOrder{}).
			Where("out_trade_no = ? AND status = ?", tradeNo, WechatPayOrderStatusPending).
			Updates(map[string]interface{}{"status": WechatPayOrderStatusClosed, "updated_at": common.GetTimestamp()}).Error; err != nil {
			return err
		}
		return tx.Model(&SubscriptionOrder{}).
			Where("trade_no = ? AND payment_provider = ? AND status = ?", tradeNo, PaymentProviderWechatSubscription, common.TopUpStatusPending).
			Updates(map[string]interface{}{"status": common.TopUpStatusExpired, "complete_time": common.GetTimestamp()}).Error
	})
}

// CompleteWechatPaySubscription first settles the common subscription order,
// then marks the Native QR child order. The operation is retry-safe: a retry
// after a process interruption sees the already-completed subscription order
// and only finishes the child-order update.
func CompleteWechatPaySubscription(completion WechatPayCompletion, providerPayload string) (bool, error) {
	if completion.EventID == "" || completion.OutTradeNo == "" || completion.TransactionID == "" {
		return false, errors.New("微信支付通知缺少必要标识")
	}
	order, err := GetSubscriptionWechatPayOrderByTradeNo(completion.OutTradeNo)
	if err != nil {
		return false, err
	}
	if order.AmountFen != completion.AmountFen || order.Currency != completion.Currency {
		return false, errors.New("微信支付通知金额或币种与本地订单不一致")
	}
	if order.WechatTransactionId != nil && *order.WechatTransactionId != completion.TransactionID {
		return false, errors.New("微信支付交易号与本地订单不一致")
	}
	if order.Status == WechatPayOrderStatusCredited {
		return false, nil
	}
	if order.Status != WechatPayOrderStatusPending {
		return false, errors.New("微信支付本地订单状态不允许入账")
	}

	if err := CompleteSubscriptionOrder(completion.OutTradeNo, providerPayload, PaymentProviderWechatSubscription, PaymentMethodWechatNative); err != nil {
		return false, err
	}

	credited := false
	err = DB.Transaction(func(tx *gorm.DB) error {
		var current SubscriptionWechatPayOrder
		if err := lockForUpdate(tx).Where("out_trade_no = ?", completion.OutTradeNo).First(&current).Error; err != nil {
			return err
		}
		if current.Status == WechatPayOrderStatusCredited {
			return nil
		}
		if current.Status != WechatPayOrderStatusPending {
			return errors.New("微信支付本地订单状态不允许入账")
		}
		transactionID := completion.TransactionID
		successTimestamp := completion.SuccessTime.Unix()
		if successTimestamp <= 0 {
			successTimestamp = common.GetTimestamp()
		}
		if err := tx.Model(&current).Updates(map[string]interface{}{
			"status":                WechatPayOrderStatusCredited,
			"wechat_transaction_id": &transactionID,
			"success_time":          successTimestamp,
			"updated_at":            common.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
		notification := &WechatPayNotification{
			EventId:          completion.EventID,
			OutTradeNo:       completion.OutTradeNo,
			BodyDigest:       completion.BodyDigest,
			ProcessingStatus: "processed",
			ReceivedAt:       common.GetTimestamp(),
			ProcessedAt:      common.GetTimestamp(),
		}
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(notification).Error; err != nil {
			return err
		}
		credited = true
		return nil
	})
	return credited, err
}

func UpdateWechatPayPrepayResult(tradeNo string, codeURL string) error {
	return DB.Model(&WechatPayOrder{}).
		Where("out_trade_no = ? AND status = ?", tradeNo, WechatPayOrderStatusPending).
		Updates(map[string]interface{}{"code_url": codeURL, "updated_at": common.GetTimestamp()}).Error
}

func MarkWechatPayOrderFailed(tradeNo string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&WechatPayOrder{}).
			Where("out_trade_no = ? AND status = ?", tradeNo, WechatPayOrderStatusPending).
			Updates(map[string]interface{}{"status": WechatPayOrderStatusFailed, "updated_at": common.GetTimestamp()}).Error; err != nil {
			return err
		}
		return tx.Model(&TopUp{}).
			Where("trade_no = ? AND payment_provider = ? AND status = ?", tradeNo, PaymentProviderWechatNative, common.TopUpStatusPending).
			Update("status", common.TopUpStatusFailed).Error
	})
}

func ClaimWechatPayOrderCheck(tradeNo string, minimumInterval time.Duration) (bool, error) {
	now := common.GetTimestamp()
	cutoff := now - int64(minimumInterval/time.Second)
	result := DB.Model(&WechatPayOrder{}).
		Where("out_trade_no = ? AND status = ? AND last_checked_at <= ?", tradeNo, WechatPayOrderStatusPending, cutoff).
		Update("last_checked_at", now)
	return result.RowsAffected == 1, result.Error
}

func CloseWechatPayOrder(tradeNo string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&WechatPayOrder{}).
			Where("out_trade_no = ? AND status = ?", tradeNo, WechatPayOrderStatusPending).
			Updates(map[string]interface{}{"status": WechatPayOrderStatusClosed, "updated_at": common.GetTimestamp()})
		if result.Error != nil || result.RowsAffected == 0 {
			return result.Error
		}
		return tx.Model(&TopUp{}).
			Where("trade_no = ? AND payment_provider = ? AND status = ?", tradeNo, PaymentProviderWechatNative, common.TopUpStatusPending).
			Update("status", common.TopUpStatusExpired).Error
	})
}

func CompleteWechatPayTopUp(completion WechatPayCompletion) (bool, error) {
	if completion.EventID == "" || completion.OutTradeNo == "" || completion.TransactionID == "" {
		return false, errors.New("微信支付通知缺少必要标识")
	}

	credited := false
	var creditedUserID int
	var creditedQuota int
	var affiliateReward *affiliateTopUpReward
	err := DB.Transaction(func(tx *gorm.DB) error {
		notification := WechatPayNotification{
			EventId:          completion.EventID,
			OutTradeNo:       completion.OutTradeNo,
			BodyDigest:       completion.BodyDigest,
			ProcessingStatus: "processing",
			ReceivedAt:       common.GetTimestamp(),
		}
		result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&notification)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}

		order := &WechatPayOrder{}
		if err := lockForUpdate(tx).Where("out_trade_no = ?", completion.OutTradeNo).First(order).Error; err != nil {
			return err
		}
		if order.AmountFen != completion.AmountFen || order.Currency != completion.Currency {
			return errors.New("微信支付通知金额或币种与本地订单不一致")
		}
		if order.WechatTransactionId != nil && *order.WechatTransactionId != completion.TransactionID {
			return errors.New("微信支付交易号与本地订单不一致")
		}
		if order.Status == WechatPayOrderStatusCredited {
			return tx.Model(&notification).Updates(map[string]interface{}{
				"processing_status": "duplicate",
				"processed_at":      common.GetTimestamp(),
			}).Error
		}
		if order.Status != WechatPayOrderStatusPending {
			return errors.New("微信支付本地订单状态不允许入账")
		}

		topUp := &TopUp{}
		if err := lockForUpdate(tx).Where("id = ?", order.TopUpId).First(topUp).Error; err != nil {
			return err
		}
		if topUp.PaymentProvider != PaymentProviderWechatNative || topUp.Status != common.TopUpStatusPending {
			return ErrPaymentMethodMismatch
		}

		quotaToAdd := order.CreditQuota
		if quotaToAdd <= 0 || quotaToAdd > common.MaxQuota {
			return errors.New("微信支付充值额度无效")
		}

		transactionID := completion.TransactionID
		completeTimestamp := completion.SuccessTime.Unix()
		if completeTimestamp <= 0 {
			completeTimestamp = common.GetTimestamp()
		}
		if err := tx.Model(order).Updates(map[string]interface{}{
			"status":                WechatPayOrderStatusCredited,
			"wechat_transaction_id": &transactionID,
			"success_time":          completeTimestamp,
			"updated_at":            common.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(topUp).Updates(map[string]interface{}{
			"complete_time": completeTimestamp,
			"status":        common.TopUpStatusSuccess,
		}).Error; err != nil {
			return err
		}
		if err := creditUserTopUpQuota(tx, topUp.UserId, quotaToAdd); err != nil {
			return err
		}
		reward, rewardErr := creditAffiliateTopUpReward(tx, topUp.UserId, quotaToAdd)
		if rewardErr != nil {
			return rewardErr
		}
		affiliateReward = reward
		if err := tx.Model(&notification).Updates(map[string]interface{}{
			"processing_status": "processed",
			"processed_at":      common.GetTimestamp(),
		}).Error; err != nil {
			return err
		}

		credited = true
		creditedUserID = topUp.UserId
		creditedQuota = quotaToAdd
		return nil
	})
	if err != nil {
		return false, err
	}
	if credited {
		recordAffiliateTopUpReward(affiliateReward)
		RecordTopupLog(creditedUserID, fmt.Sprintf("微信支付充值成功，充值额度: %v", logger.FormatQuota(creditedQuota)), "", PaymentMethodWechatNative, PaymentProviderWechatNative)
	}
	return credited, nil
}
