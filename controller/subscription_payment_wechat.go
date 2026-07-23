package controller

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
	"gorm.io/gorm"
)

type SubscriptionWechatPayRequest struct {
	PlanId          int    `json:"plan_id"`
	ClientRequestID string `json:"client_request_id"`
}

func writeSubscriptionWechatPayResponse(c *gin.Context, order *model.SubscriptionWechatPayOrder) {
	common.ApiSuccess(c, gin.H{
		"trade_no":   order.OutTradeNo,
		"code_url":   order.CodeUrl,
		"amount_fen": order.AmountFen,
		"expires_at": order.ExpiresAt,
		"status":     order.Status,
	})
}

func SubscriptionRequestWechatPay(c *gin.Context) {
	if !isWechatPayTopUpEnabled() {
		common.ApiErrorMsg(c, "微信支付暂不可用")
		return
	}
	var req SubscriptionWechatPayRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 || !wechatPayClientRequestIDPattern.MatchString(req.ClientRequestID) {
		common.ApiErrorMsg(c, "参数错误")
		return
	}

	userID := c.GetInt("id")
	existingOrder, err := model.GetSubscriptionWechatPayOrderByClientRequest(userID, req.ClientRequestID)
	if err == nil {
		writeSubscriptionWechatPayResponse(c, existingOrder)
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		common.ApiErrorMsg(c, "创建微信支付订单失败")
		return
	}

	plan, err := model.GetSubscriptionPlanById(req.PlanId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !plan.Enabled {
		common.ApiErrorMsg(c, "套餐未启用")
		return
	}
	if plan.Currency != "CNY" {
		common.ApiErrorMsg(c, "微信支付仅支持人民币套餐")
		return
	}
	amountFen, err := wechatPayAmountFen(plan.PriceAmount)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	runtime, err := wechatPayRuntimeLoader(c.Request.Context())
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("微信支付初始化失败 error=%q", err.Error()))
		common.ApiErrorMsg(c, "微信支付配置不可用")
		return
	}

	now := time.Now()
	expiresAt := now.Add(wechatPayOrderLifetime)
	tradeNo := fmt.Sprintf("WS%d%s", now.UnixMilli(), common.GetRandomString(8))
	subscriptionOrder := &model.SubscriptionOrder{
		UserId:          userID,
		PlanId:          plan.Id,
		Money:           plan.PriceAmount,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodWechatNative,
		PaymentProvider: model.PaymentProviderWechatSubscription,
		Status:          common.TopUpStatusPending,
		CreateTime:      now.Unix(),
	}
	order := &model.SubscriptionWechatPayOrder{
		UserId:          userID,
		ClientRequestId: req.ClientRequestID,
		OutTradeNo:      tradeNo,
		AmountFen:       amountFen,
		Currency:        "CNY",
		Status:          model.WechatPayOrderStatusPending,
		ExpiresAt:       expiresAt.Unix(),
		CreatedAt:       now.Unix(),
		UpdatedAt:       now.Unix(),
	}
	if err := model.CreateSubscriptionWechatPayOrder(subscriptionOrder, order); err != nil {
		if duplicateOrder, lookupErr := model.GetSubscriptionWechatPayOrderByClientRequest(userID, req.ClientRequestID); lookupErr == nil {
			writeSubscriptionWechatPayResponse(c, duplicateOrder)
			return
		}
		if errors.Is(err, model.ErrSubscriptionPurchaseLimit) {
			common.ApiErrorMsg(c, err.Error())
			return
		}
		logger.LogError(c.Request.Context(), fmt.Sprintf("微信支付创建套餐订单失败 user_id=%d plan_id=%d error=%q", userID, plan.Id, err.Error()))
		common.ApiErrorMsg(c, "创建微信支付订单失败")
		return
	}

	description := fmt.Sprintf("MeiMei API 套餐：%s", plan.Title)
	codeURL, err := prepayWechatNativeOrder(c.Request.Context(), runtime, tradeNo, amountFen, description, expiresAt)
	if err != nil {
		_ = model.MarkSubscriptionWechatPayOrderFailed(tradeNo)
		logger.LogError(c.Request.Context(), fmt.Sprintf("微信支付套餐预下单失败 user_id=%d trade_no=%s error=%q", userID, tradeNo, err.Error()))
		common.ApiErrorMsg(c, "微信支付下单失败")
		return
	}
	order.CodeUrl = codeURL
	if err := model.UpdateSubscriptionWechatPayPrepayResult(tradeNo, codeURL); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("微信支付保存套餐二维码失败 user_id=%d trade_no=%s error=%q", userID, tradeNo, err.Error()))
		common.ApiErrorMsg(c, "保存微信支付订单失败")
		return
	}
	writeSubscriptionWechatPayResponse(c, order)
}

func GetSubscriptionWechatPayStatus(c *gin.Context) {
	order, err := model.GetSubscriptionWechatPayOrderForUser(c.GetInt("id"), c.Param("trade_no"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.Status(http.StatusNotFound)
			return
		}
		common.ApiErrorMsg(c, "查询微信支付订单失败")
		return
	}
	if order.Status == model.WechatPayOrderStatusPending {
		claimed, claimErr := model.ClaimSubscriptionWechatPayOrderCheck(order.OutTradeNo, 10*time.Second)
		if claimErr != nil {
			common.ApiErrorMsg(c, "查询微信支付订单失败")
			return
		}
		if claimed {
			reconcileSubscriptionWechatPayOrder(c.Request.Context(), order)
			if refreshedOrder, refreshErr := model.GetSubscriptionWechatPayOrderForUser(c.GetInt("id"), order.OutTradeNo); refreshErr == nil {
				order = refreshedOrder
			}
		}
	}
	common.ApiSuccess(c, gin.H{
		"trade_no":   order.OutTradeNo,
		"amount_fen": order.AmountFen,
		"expires_at": order.ExpiresAt,
		"status":     order.Status,
	})
}

func reconcileSubscriptionWechatPayOrder(ctx context.Context, order *model.SubscriptionWechatPayOrder) {
	runtime, err := wechatPayRuntimeLoader(ctx)
	if err != nil {
		return
	}
	transaction, _, err := runtime.nativeService.QueryOrderByOutTradeNo(ctx, native.QueryOrderByOutTradeNoRequest{
		OutTradeNo: core.String(order.OutTradeNo),
		Mchid:      core.String(runtime.config.MchID),
	})
	if err != nil || transaction == nil || transaction.TradeState == nil {
		return
	}

	switch *transaction.TradeState {
	case "SUCCESS":
		if transaction.OutTradeNo == nil || transaction.TransactionId == nil || transaction.Amount == nil ||
			transaction.Amount.Total == nil || transaction.Amount.Currency == nil || transaction.Appid == nil ||
			transaction.Mchid == nil || transaction.SuccessTime == nil ||
			*transaction.Appid != runtime.config.AppID || *transaction.Mchid != runtime.config.MchID {
			logger.LogWarn(ctx, fmt.Sprintf("微信支付套餐主动查单字段或身份不匹配 trade_no=%s", order.OutTradeNo))
			return
		}
		successTime, parseErr := time.Parse(time.RFC3339, *transaction.SuccessTime)
		if parseErr != nil {
			return
		}
		transactionDigest := fmt.Sprintf("%x", sha256.Sum256([]byte(*transaction.TransactionId)))
		completion := model.WechatPayCompletion{
			EventID:       "query:" + *transaction.TransactionId,
			OutTradeNo:    *transaction.OutTradeNo,
			TransactionID: *transaction.TransactionId,
			AmountFen:     *transaction.Amount.Total,
			Currency:      *transaction.Amount.Currency,
			SuccessTime:   successTime,
			BodyDigest:    transactionDigest,
		}
		payload := fmt.Sprintf("wechat_transaction_id=%s query_digest=%s", completion.TransactionID, transactionDigest)
		if _, err := model.CompleteWechatPaySubscription(completion, payload); err != nil {
			logger.LogError(ctx, fmt.Sprintf("微信支付套餐主动查单入账失败 trade_no=%s error=%q", order.OutTradeNo, err.Error()))
		}
	case "CLOSED", "REVOKED", "PAYERROR":
		_ = model.CloseSubscriptionWechatPayOrder(order.OutTradeNo)
	case "NOTPAY":
		if time.Now().Unix() < order.ExpiresAt {
			return
		}
		_, closeErr := runtime.nativeService.CloseOrder(ctx, native.CloseOrderRequest{
			OutTradeNo: core.String(order.OutTradeNo),
			Mchid:      core.String(runtime.config.MchID),
		})
		if closeErr == nil {
			_ = model.CloseSubscriptionWechatPayOrder(order.OutTradeNo)
		}
	}
}

func completeWechatPayBusinessOrder(completion model.WechatPayCompletion) error {
	if _, err := model.GetSubscriptionWechatPayOrderByTradeNo(completion.OutTradeNo); err == nil {
		payload := fmt.Sprintf("wechat_transaction_id=%s body_digest=%s", completion.TransactionID, completion.BodyDigest)
		_, completionErr := model.CompleteWechatPaySubscription(completion, payload)
		return completionErr
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	_, err := model.CompleteWechatPayTopUp(completion)
	return err
}
