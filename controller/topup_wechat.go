package controller

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/core/option"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
	"github.com/wechatpay-apiv3/wechatpay-go/utils"
	"gorm.io/gorm"
)

const (
	wechatPayOrderLifetime = 15 * time.Minute
	wechatPayMaxAmountFen  = int64(100_000_000)
	wechatPayNotifyMaxBody = int64(64 * 1024)
)

var wechatPayClientRequestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)

type wechatPayRuntime struct {
	config        setting.WechatPayConfig
	nativeService wechatNativeService
	notifyHandler wechatNotifyHandler
}

type wechatNativeService interface {
	Prepay(context.Context, native.PrepayRequest) (*native.PrepayResponse, *core.APIResult, error)
	QueryOrderByOutTradeNo(context.Context, native.QueryOrderByOutTradeNoRequest) (*payments.Transaction, *core.APIResult, error)
	CloseOrder(context.Context, native.CloseOrderRequest) (*core.APIResult, error)
}

type wechatNotifyHandler interface {
	ParseNotifyRequest(context.Context, *http.Request, interface{}) (*notify.Request, error)
}

var wechatPayRuntimeLoader = getWechatPayRuntime
var wechatPayWebhookEnabled = isWechatPayWebhookEnabled

var wechatPayRuntimeCache struct {
	sync.Mutex
	runtime *wechatPayRuntime
}

type WechatNativePayRequest struct {
	Amount          int64  `json:"amount"`
	ClientRequestID string `json:"client_request_id"`
}

func getWechatPayRuntime(ctx context.Context) (*wechatPayRuntime, error) {
	config := setting.GetWechatPayConfig()
	if err := config.Validate(); err != nil {
		return nil, err
	}

	wechatPayRuntimeCache.Lock()
	defer wechatPayRuntimeCache.Unlock()
	if wechatPayRuntimeCache.runtime != nil && wechatPayRuntimeCache.runtime.config == config {
		return wechatPayRuntimeCache.runtime, nil
	}

	merchantPrivateKey, err := utils.LoadPrivateKeyWithPath(config.MerchantPrivateKeyPath)
	if err != nil {
		return nil, errors.New("微信支付商户私钥加载失败")
	}
	wechatPayPublicKey, err := utils.LoadPublicKeyWithPath(config.PublicKeyPath)
	if err != nil {
		return nil, errors.New("微信支付公钥加载失败")
	}

	httpClient := &http.Client{Timeout: 15 * time.Second}
	client, err := core.NewClient(ctx,
		option.WithHTTPClient(httpClient),
		option.WithWechatPayPublicKeyAuthCipher(
			config.MchID,
			config.MerchantSerialNo,
			merchantPrivateKey,
			config.PublicKeyID,
			wechatPayPublicKey,
		),
	)
	if err != nil {
		return nil, errors.New("微信支付客户端初始化失败")
	}
	verifier := verifiers.NewSHA256WithRSAPubkeyVerifier(config.PublicKeyID, *wechatPayPublicKey)
	notifyHandler, err := notify.NewRSANotifyHandler(config.APIv3Key, verifier)
	if err != nil {
		return nil, errors.New("微信支付通知处理器初始化失败")
	}

	runtime := &wechatPayRuntime{
		config:        config,
		nativeService: &native.NativeApiService{Client: client},
		notifyHandler: notifyHandler,
	}
	wechatPayRuntimeCache.runtime = runtime
	return runtime, nil
}

func wechatPayAmountFen(payMoney float64) (int64, error) {
	amountFenDecimal := decimal.NewFromFloat(payMoney).Mul(decimal.NewFromInt(100)).Round(0)
	if amountFenDecimal.LessThan(decimal.NewFromInt(1)) || amountFenDecimal.GreaterThan(decimal.NewFromInt(wechatPayMaxAmountFen)) {
		return 0, errors.New("微信支付金额超出允许范围")
	}
	return amountFenDecimal.IntPart(), nil
}

func normalizeWechatTopUpAmount(amount int64) int64 {
	if operation_setting.GetQuotaDisplayType() != operation_setting.QuotaDisplayTypeTokens {
		return amount
	}
	return decimal.NewFromInt(amount).Div(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart()
}

func wechatPayCreditQuota(amount int64) (int, error) {
	creditQuotaDecimal := decimal.NewFromInt(amount)
	if operation_setting.GetQuotaDisplayType() != operation_setting.QuotaDisplayTypeTokens {
		creditQuotaDecimal = creditQuotaDecimal.Mul(decimal.NewFromFloat(common.QuotaPerUnit))
	}
	creditQuota, clamp := common.QuotaFromDecimalChecked(creditQuotaDecimal)
	if clamp != nil || creditQuota <= 0 {
		return 0, errors.New("充值额度超出允许范围")
	}
	return creditQuota, nil
}

func writeWechatNativePayResponse(c *gin.Context, order *model.WechatPayOrder) {
	common.ApiSuccess(c, gin.H{
		"trade_no":   order.OutTradeNo,
		"code_url":   order.CodeUrl,
		"amount_fen": order.AmountFen,
		"expires_at": order.ExpiresAt,
		"status":     order.Status,
	})
}

func prepayWechatNativeOrder(ctx context.Context, runtime *wechatPayRuntime, tradeNo string, amountFen int64, description string, expiresAt time.Time) (string, error) {
	response, _, err := runtime.nativeService.Prepay(ctx, native.PrepayRequest{
		Appid:       core.String(runtime.config.AppID),
		Mchid:       core.String(runtime.config.MchID),
		Description: core.String(description),
		OutTradeNo:  core.String(tradeNo),
		TimeExpire:  core.Time(expiresAt),
		NotifyUrl:   core.String(runtime.config.NotifyURL),
		Amount: &native.Amount{
			Total:    core.Int64(amountFen),
			Currency: core.String("CNY"),
		},
	})
	if err != nil || response == nil || response.CodeUrl == nil || *response.CodeUrl == "" {
		if err != nil {
			return "", err
		}
		return "", errors.New("微信支付预下单响应缺少 code_url")
	}
	return *response.CodeUrl, nil
}

func RequestWechatNativePay(c *gin.Context) {
	if !isWechatPayTopUpEnabled() {
		common.ApiErrorMsg(c, "微信支付暂不可用")
		return
	}

	var req WechatNativePayRequest
	if err := c.ShouldBindJSON(&req); err != nil || !wechatPayClientRequestIDPattern.MatchString(req.ClientRequestID) {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if req.Amount < getMinTopup() {
		common.ApiErrorMsg(c, fmt.Sprintf("充值数量不能小于 %d", getMinTopup()))
		return
	}

	userID := c.GetInt("id")
	existingOrder, err := model.GetWechatPayOrderByClientRequest(userID, req.ClientRequestID)
	if err == nil {
		writeWechatNativePayResponse(c, existingOrder)
		return
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		common.ApiErrorMsg(c, "创建微信支付订单失败")
		return
	}

	group, err := model.GetUserGroup(userID, true)
	if err != nil {
		common.ApiErrorMsg(c, "获取用户分组失败")
		return
	}
	payMoney := getPayMoney(req.Amount, group)
	amountFen, err := wechatPayAmountFen(payMoney)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	normalizedAmount := normalizeWechatTopUpAmount(req.Amount)
	if normalizedAmount <= 0 {
		common.ApiErrorMsg(c, "充值额度无效")
		return
	}
	creditQuota, err := wechatPayCreditQuota(req.Amount)
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
	tradeNo := fmt.Sprintf("WR%d%s", now.UnixMilli(), common.GetRandomString(8))
	topUp := &model.TopUp{
		UserId:          userID,
		Amount:          normalizedAmount,
		Money:           decimal.NewFromInt(amountFen).Div(decimal.NewFromInt(100)).InexactFloat64(),
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodWechatNative,
		PaymentProvider: model.PaymentProviderWechatNative,
		CreateTime:      now.Unix(),
		Status:          common.TopUpStatusPending,
	}
	order := &model.WechatPayOrder{
		UserId:          userID,
		ClientRequestId: req.ClientRequestID,
		OutTradeNo:      tradeNo,
		AmountFen:       amountFen,
		CreditQuota:     creditQuota,
		Currency:        "CNY",
		Status:          model.WechatPayOrderStatusPending,
		ExpiresAt:       expiresAt.Unix(),
		CreatedAt:       now.Unix(),
		UpdatedAt:       now.Unix(),
	}
	if err = model.CreateWechatPayTopUp(topUp, order); err != nil {
		if duplicateOrder, lookupErr := model.GetWechatPayOrderByClientRequest(userID, req.ClientRequestID); lookupErr == nil {
			writeWechatNativePayResponse(c, duplicateOrder)
			return
		}
		logger.LogError(c.Request.Context(), fmt.Sprintf("微信支付创建本地订单失败 user_id=%d error=%q", userID, err.Error()))
		common.ApiErrorMsg(c, "创建微信支付订单失败")
		return
	}

	description := fmt.Sprintf("莓莓 API 额度充值（%.2f元）", topUp.Money)
	codeURL, err := prepayWechatNativeOrder(c.Request.Context(), runtime, tradeNo, amountFen, description, expiresAt)
	if err != nil {
		_ = model.MarkWechatPayOrderFailed(tradeNo)
		logger.LogError(c.Request.Context(), fmt.Sprintf("微信支付预下单失败 user_id=%d trade_no=%s error=%q", userID, tradeNo, err.Error()))
		common.ApiErrorMsg(c, "微信支付下单失败")
		return
	}
	order.CodeUrl = codeURL
	if err = model.UpdateWechatPayPrepayResult(tradeNo, order.CodeUrl); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("微信支付保存二维码失败 user_id=%d trade_no=%s error=%q", userID, tradeNo, err.Error()))
		common.ApiErrorMsg(c, "保存微信支付订单失败")
		return
	}
	writeWechatNativePayResponse(c, order)
}

func GetWechatNativePayStatus(c *gin.Context) {
	order, err := model.GetWechatPayOrderForUser(c.GetInt("id"), c.Param("trade_no"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.Status(http.StatusNotFound)
			return
		}
		common.ApiErrorMsg(c, "查询微信支付订单失败")
		return
	}
	if order.Status == model.WechatPayOrderStatusPending {
		claimed, claimErr := model.ClaimWechatPayOrderCheck(order.OutTradeNo, 10*time.Second)
		if claimErr != nil {
			common.ApiErrorMsg(c, "查询微信支付订单失败")
			return
		}
		if claimed {
			reconcileWechatPayOrder(c.Request.Context(), order)
			if refreshedOrder, refreshErr := model.GetWechatPayOrderForUser(c.GetInt("id"), order.OutTradeNo); refreshErr == nil {
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

func reconcileWechatPayOrder(ctx context.Context, order *model.WechatPayOrder) {
	runtime, err := wechatPayRuntimeLoader(ctx)
	if err != nil {
		return
	}
	reconcileWechatPayOrderWithRuntime(ctx, order, runtime)
}

func reconcileWechatPayOrderWithRuntime(ctx context.Context, order *model.WechatPayOrder, runtime *wechatPayRuntime) {
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
			logger.LogWarn(ctx, fmt.Sprintf("微信支付主动查单字段或身份不匹配 trade_no=%s", order.OutTradeNo))
			return
		}
		successTime, parseErr := time.Parse(time.RFC3339, *transaction.SuccessTime)
		if parseErr != nil {
			return
		}
		transactionDigest := fmt.Sprintf("%x", sha256.Sum256([]byte(*transaction.TransactionId)))
		_, completionErr := model.CompleteWechatPayTopUp(model.WechatPayCompletion{
			EventID:       "query:" + *transaction.TransactionId,
			OutTradeNo:    *transaction.OutTradeNo,
			TransactionID: *transaction.TransactionId,
			AmountFen:     *transaction.Amount.Total,
			Currency:      *transaction.Amount.Currency,
			SuccessTime:   successTime,
			BodyDigest:    transactionDigest,
		})
		if completionErr != nil {
			logger.LogError(ctx, fmt.Sprintf("微信支付主动查单入账失败 trade_no=%s error=%q", order.OutTradeNo, completionErr.Error()))
		}
	case "CLOSED", "REVOKED", "PAYERROR":
		_ = model.CloseWechatPayOrder(order.OutTradeNo)
	case "NOTPAY":
		if time.Now().Unix() < order.ExpiresAt {
			return
		}
		_, closeErr := runtime.nativeService.CloseOrder(ctx, native.CloseOrderRequest{
			OutTradeNo: core.String(order.OutTradeNo),
			Mchid:      core.String(runtime.config.MchID),
		})
		if closeErr == nil {
			_ = model.CloseWechatPayOrder(order.OutTradeNo)
		}
	}
}

func WechatPayNotify(c *gin.Context) {
	if !wechatPayWebhookEnabled() {
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}
	runtime, err := wechatPayRuntimeLoader(c.Request.Context())
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("微信支付回调初始化失败 error=%q", err.Error()))
		c.AbortWithStatus(http.StatusServiceUnavailable)
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, wechatPayNotifyMaxBody)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	bodyDigest := fmt.Sprintf("%x", sha256.Sum256(body))

	transaction := &payments.Transaction{}
	notifyRequest, err := runtime.notifyHandler.ParseNotifyRequest(c.Request.Context(), c.Request, transaction)
	if err != nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("微信支付回调验签或解密失败 body_digest=%s error=%q", bodyDigest, err.Error()))
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	if notifyRequest.EventType != "TRANSACTION.SUCCESS" || transaction.TradeState == nil || *transaction.TradeState != "SUCCESS" ||
		transaction.OutTradeNo == nil || transaction.TransactionId == nil || transaction.Amount == nil ||
		transaction.Amount.Total == nil || transaction.Amount.Currency == nil || transaction.Appid == nil || transaction.Mchid == nil ||
		transaction.SuccessTime == nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("微信支付回调字段不完整 event_id=%s body_digest=%s", notifyRequest.ID, bodyDigest))
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	if *transaction.Appid != runtime.config.AppID || *transaction.Mchid != runtime.config.MchID {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("微信支付回调商户身份不匹配 event_id=%s trade_no=%s", notifyRequest.ID, *transaction.OutTradeNo))
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}
	successTime, err := time.Parse(time.RFC3339, *transaction.SuccessTime)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	_, err = model.CompleteWechatPayTopUp(model.WechatPayCompletion{
		EventID:       notifyRequest.ID,
		OutTradeNo:    *transaction.OutTradeNo,
		TransactionID: *transaction.TransactionId,
		AmountFen:     *transaction.Amount.Total,
		Currency:      *transaction.Amount.Currency,
		SuccessTime:   successTime,
		BodyDigest:    bodyDigest,
	})
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("微信支付回调入账失败 event_id=%s trade_no=%s error=%q", notifyRequest.ID, *transaction.OutTradeNo, err.Error()))
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}
	c.AbortWithStatus(http.StatusNoContent)
}
