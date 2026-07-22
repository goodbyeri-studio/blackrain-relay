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
	PaymentMethodAlipayPrecreate   = "alipay_precreate"
	PaymentProviderAlipayPrecreate = "alipay_precreate"

	AlipayOrderStatusPending  = "pending"
	AlipayOrderStatusCredited = "credited"
	AlipayOrderStatusFailed   = "failed"
	AlipayOrderStatusClosed   = "closed"
)

type AlipayOrder struct {
	Id              int     `json:"id"`
	TopUpId         int     `json:"topup_id" gorm:"uniqueIndex"`
	UserId          int     `json:"user_id" gorm:"index;uniqueIndex:idx_alipay_user_request"`
	ClientRequestId string  `json:"client_request_id" gorm:"type:varchar(64);uniqueIndex:idx_alipay_user_request"`
	OutTradeNo      string  `json:"out_trade_no" gorm:"type:varchar(64);uniqueIndex"`
	AmountFen       int64   `json:"amount_fen"`
	CreditQuota     int     `json:"credit_quota"`
	Currency        string  `json:"currency" gorm:"type:varchar(8)"`
	Status          string  `json:"status" gorm:"type:varchar(24);index"`
	QRCode          string  `json:"-" gorm:"type:text"`
	AlipayTradeNo   *string `json:"-" gorm:"type:varchar(64);uniqueIndex"`
	ExpiresAt       int64   `json:"expires_at"`
	SuccessTime     int64   `json:"success_time"`
	CreatedAt       int64   `json:"created_at"`
	UpdatedAt       int64   `json:"updated_at"`
	LastCheckedAt   int64   `json:"last_checked_at"`
}

type AlipayNotification struct {
	Id               int    `json:"id"`
	EventId          string `json:"event_id" gorm:"type:varchar(128);uniqueIndex"`
	OutTradeNo       string `json:"out_trade_no" gorm:"type:varchar(64);index"`
	BodyDigest       string `json:"body_digest" gorm:"type:varchar(64)"`
	ProcessingStatus string `json:"processing_status" gorm:"type:varchar(24)"`
	ReceivedAt       int64  `json:"received_at"`
	ProcessedAt      int64  `json:"processed_at"`
}

type AlipayCompletion struct {
	EventID     string
	OutTradeNo  string
	TradeNo     string
	AmountFen   int64
	Currency    string
	SuccessTime time.Time
	BodyDigest  string
}

func CreateAlipayTopUp(topUp *TopUp, order *AlipayOrder) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(topUp).Error; err != nil {
			return err
		}
		order.TopUpId = topUp.Id
		return tx.Create(order).Error
	})
}

func GetAlipayOrderByClientRequest(userID int, clientRequestID string) (*AlipayOrder, error) {
	var order AlipayOrder
	if err := DB.Where("user_id = ? AND client_request_id = ?", userID, clientRequestID).First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func GetAlipayOrderForUser(userID int, tradeNo string) (*AlipayOrder, error) {
	var order AlipayOrder
	if err := DB.Where("user_id = ? AND out_trade_no = ?", userID, tradeNo).First(&order).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

func UpdateAlipayPrecreateResult(tradeNo string, qrCode string) error {
	return DB.Model(&AlipayOrder{}).
		Where("out_trade_no = ? AND status = ?", tradeNo, AlipayOrderStatusPending).
		Updates(map[string]interface{}{"qr_code": qrCode, "updated_at": common.GetTimestamp()}).Error
}

func MarkAlipayOrderFailed(tradeNo string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&AlipayOrder{}).
			Where("out_trade_no = ? AND status = ?", tradeNo, AlipayOrderStatusPending).
			Updates(map[string]interface{}{"status": AlipayOrderStatusFailed, "updated_at": common.GetTimestamp()}).Error; err != nil {
			return err
		}
		return tx.Model(&TopUp{}).
			Where("trade_no = ? AND payment_provider = ? AND status = ?", tradeNo, PaymentProviderAlipayPrecreate, common.TopUpStatusPending).
			Update("status", common.TopUpStatusFailed).Error
	})
}

func ClaimAlipayOrderCheck(tradeNo string, minimumInterval time.Duration) (bool, error) {
	now := common.GetTimestamp()
	cutoff := now - int64(minimumInterval/time.Second)
	result := DB.Model(&AlipayOrder{}).
		Where("out_trade_no = ? AND status = ? AND last_checked_at <= ?", tradeNo, AlipayOrderStatusPending, cutoff).
		Update("last_checked_at", now)
	return result.RowsAffected == 1, result.Error
}

func CloseAlipayOrder(tradeNo string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&AlipayOrder{}).
			Where("out_trade_no = ? AND status = ?", tradeNo, AlipayOrderStatusPending).
			Updates(map[string]interface{}{"status": AlipayOrderStatusClosed, "updated_at": common.GetTimestamp()})
		if result.Error != nil || result.RowsAffected == 0 {
			return result.Error
		}
		return tx.Model(&TopUp{}).
			Where("trade_no = ? AND payment_provider = ? AND status = ?", tradeNo, PaymentProviderAlipayPrecreate, common.TopUpStatusPending).
			Update("status", common.TopUpStatusExpired).Error
	})
}

func CompleteAlipayTopUp(completion AlipayCompletion) (bool, error) {
	if completion.EventID == "" || completion.OutTradeNo == "" || completion.TradeNo == "" {
		return false, errors.New("支付宝通知缺少必要标识")
	}
	if completion.Currency != "CNY" {
		return false, errors.New("支付宝通知币种不支持")
	}

	credited := false
	var creditedUserID int
	var creditedQuota int
	err := DB.Transaction(func(tx *gorm.DB) error {
		notification := AlipayNotification{
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

		order := &AlipayOrder{}
		if err := lockForUpdate(tx).Where("out_trade_no = ?", completion.OutTradeNo).First(order).Error; err != nil {
			return err
		}
		if order.AmountFen != completion.AmountFen || order.Currency != completion.Currency {
			return errors.New("支付宝通知金额或币种与本地订单不一致")
		}
		if order.AlipayTradeNo != nil && *order.AlipayTradeNo != completion.TradeNo {
			return errors.New("支付宝交易号与本地订单不一致")
		}
		if order.Status == AlipayOrderStatusCredited {
			return tx.Model(&notification).Updates(map[string]interface{}{
				"processing_status": "duplicate",
				"processed_at":      common.GetTimestamp(),
			}).Error
		}
		if order.Status != AlipayOrderStatusPending {
			return errors.New("支付宝本地订单状态不允许入账")
		}

		topUp := &TopUp{}
		if err := lockForUpdate(tx).Where("id = ?", order.TopUpId).First(topUp).Error; err != nil {
			return err
		}
		if topUp.PaymentProvider != PaymentProviderAlipayPrecreate || topUp.Status != common.TopUpStatusPending {
			return ErrPaymentMethodMismatch
		}

		quotaToAdd := order.CreditQuota
		if quotaToAdd <= 0 || quotaToAdd > common.MaxQuota {
			return errors.New("支付宝充值额度无效")
		}

		tradeNo := completion.TradeNo
		completeTimestamp := completion.SuccessTime.Unix()
		if completeTimestamp <= 0 {
			completeTimestamp = common.GetTimestamp()
		}
		if err := tx.Model(order).Updates(map[string]interface{}{
			"status":          AlipayOrderStatusCredited,
			"alipay_trade_no": &tradeNo,
			"success_time":    completeTimestamp,
			"updated_at":      common.GetTimestamp(),
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(topUp).Updates(map[string]interface{}{
			"complete_time": completeTimestamp,
			"status":        common.TopUpStatusSuccess,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&User{}).Where("id = ?", topUp.UserId).
			Update("quota", gorm.Expr("quota + ?", quotaToAdd)).Error; err != nil {
			return err
		}
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
		RecordTopupLog(creditedUserID, fmt.Sprintf("支付宝扫码充值成功，充值额度: %v", logger.FormatQuota(creditedQuota)), "", PaymentMethodAlipayPrecreate, PaymentProviderAlipayPrecreate)
	}
	return credited, nil
}
