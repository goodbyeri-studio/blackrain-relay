package controller

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	alipayOrderLifetime        = 15 * time.Minute
	alipayMaxAmountFen         = int64(100_000_000)
	alipayNotifyMaxBody        = int64(64 * 1024)
	alipayGatewayResponseLimit = int64(1 * 1024 * 1024)
)

var alipayClientRequestIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{16,64}$`)

var errAlipayTradeNotExist = errors.New("支付宝交易不存在")

type alipayRuntime struct {
	config     setting.AlipayConfig
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	httpClient *http.Client
}

var alipayRuntimeCache struct {
	sync.Mutex
	runtime *alipayRuntime
}

type AlipayPrecreateRequest struct {
	Amount          int64  `json:"amount"`
	ClientRequestID string `json:"client_request_id"`
}

type alipayPrecreateBizContent struct {
	OutTradeNo     string `json:"out_trade_no"`
	TotalAmount    string `json:"total_amount"`
	Subject        string `json:"subject"`
	TimeoutExpress string `json:"timeout_express"`
}

type alipayQueryBizContent struct {
	OutTradeNo string `json:"out_trade_no"`
}

type alipayCloseBizContent struct {
	OutTradeNo string `json:"out_trade_no"`
}

type alipayPrecreateResponse struct {
	Code       string `json:"code"`
	Msg        string `json:"msg"`
	SubCode    string `json:"sub_code"`
	SubMsg     string `json:"sub_msg"`
	OutTradeNo string `json:"out_trade_no"`
	QRCode     string `json:"qr_code"`
}

type alipayQueryResponse struct {
	Code        string `json:"code"`
	Msg         string `json:"msg"`
	SubCode     string `json:"sub_code"`
	SubMsg      string `json:"sub_msg"`
	OutTradeNo  string `json:"out_trade_no"`
	TradeNo     string `json:"trade_no"`
	TradeStatus string `json:"trade_status"`
	TotalAmount string `json:"total_amount"`
	SendPayDate string `json:"send_pay_date"`
}

type alipayCloseResponse struct {
	Code       string `json:"code"`
	Msg        string `json:"msg"`
	SubCode    string `json:"sub_code"`
	SubMsg     string `json:"sub_msg"`
	OutTradeNo string `json:"out_trade_no"`
	TradeNo    string `json:"trade_no"`
}

type alipayPrecreateGatewayResponse struct {
	Response json.RawMessage `json:"alipay_trade_precreate_response"`
	Sign     string          `json:"sign"`
}

type alipayQueryGatewayResponse struct {
	Response json.RawMessage `json:"alipay_trade_query_response"`
	Sign     string          `json:"sign"`
}

type alipayCloseGatewayResponse struct {
	Response json.RawMessage `json:"alipay_trade_close_response"`
	Sign     string          `json:"sign"`
}

func getAlipayRuntime() (*alipayRuntime, error) {
	config := setting.GetAlipayConfig()
	if err := config.Validate(); err != nil {
		return nil, err
	}

	alipayRuntimeCache.Lock()
	defer alipayRuntimeCache.Unlock()
	if alipayRuntimeCache.runtime != nil && alipayRuntimeCache.runtime.config == config {
		return alipayRuntimeCache.runtime, nil
	}

	privateKey, err := loadRSAPrivateKey(config.AppPrivateKeyPath)
	if err != nil {
		return nil, errors.New("支付宝应用私钥加载失败")
	}
	publicKey, err := loadRSAPublicKey(config.AlipayPublicKeyPath)
	if err != nil {
		return nil, errors.New("支付宝公钥加载失败")
	}

	runtime := &alipayRuntime{
		config:     config,
		privateKey: privateKey,
		publicKey:  publicKey,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
	alipayRuntimeCache.runtime = runtime
	return runtime, nil
}

func loadRSAPrivateKey(path string) (*rsa.PrivateKey, error) {
	keyBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(keyBytes)
	if block == nil {
		return nil, errors.New("invalid private key pem")
	}
	if privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return privateKey, nil
	}
	parsedKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	privateKey, ok := parsedKey.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not rsa")
	}
	return privateKey, nil
}

func loadRSAPublicKey(path string) (*rsa.PublicKey, error) {
	keyBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(keyBytes)
	if block == nil {
		return nil, errors.New("invalid public key pem")
	}
	if block.Type == "CERTIFICATE" {
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		publicKey, ok := cert.PublicKey.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("certificate public key is not rsa")
		}
		return publicKey, nil
	}
	parsedKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	publicKey, ok := parsedKey.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("public key is not rsa")
	}
	return publicKey, nil
}

func alipayAmountFen(payMoney float64) (int64, error) {
	amountFenDecimal := decimal.NewFromFloat(payMoney).Mul(decimal.NewFromInt(100)).Round(0)
	if amountFenDecimal.LessThan(decimal.NewFromInt(1)) || amountFenDecimal.GreaterThan(decimal.NewFromInt(alipayMaxAmountFen)) {
		return 0, errors.New("支付宝金额超出允许范围")
	}
	return amountFenDecimal.IntPart(), nil
}

func alipayAmountYuan(amountFen int64) string {
	return decimal.NewFromInt(amountFen).Div(decimal.NewFromInt(100)).StringFixed(2)
}

func alipayAmountFenFromYuan(amountYuan string) (int64, error) {
	amount, err := decimal.NewFromString(strings.TrimSpace(amountYuan))
	if err != nil {
		return 0, err
	}
	amountFen := amount.Mul(decimal.NewFromInt(100)).Round(0)
	if amountFen.LessThan(decimal.NewFromInt(1)) || amountFen.GreaterThan(decimal.NewFromInt(alipayMaxAmountFen)) {
		return 0, errors.New("支付宝金额超出允许范围")
	}
	return amountFen.IntPart(), nil
}

func normalizeAlipayTopUpAmount(amount int64) int64 {
	if operation_setting.GetQuotaDisplayType() != operation_setting.QuotaDisplayTypeTokens {
		return amount
	}
	return decimal.NewFromInt(amount).Div(decimal.NewFromFloat(common.QuotaPerUnit)).IntPart()
}

func alipayCreditQuota(amount int64) (int, error) {
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

func alipaySignContent(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for key, value := range params {
		if key == "sign" || key == "sign_type" || value == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+params[key])
	}
	return strings.Join(parts, "&")
}

func (runtime *alipayRuntime) sign(params map[string]string) error {
	digest := sha256.Sum256([]byte(alipaySignContent(params)))
	signature, err := rsa.SignPKCS1v15(rand.Reader, runtime.privateKey, crypto.SHA256, digest[:])
	if err != nil {
		return err
	}
	params["sign"] = base64.StdEncoding.EncodeToString(signature)
	return nil
}

func (runtime *alipayRuntime) verify(params map[string]string) error {
	signatureText := params["sign"]
	if signatureText == "" {
		return errors.New("支付宝签名为空")
	}
	if signType := params["sign_type"]; signType != "" && signType != "RSA2" {
		return errors.New("支付宝签名类型不支持")
	}
	signature, err := base64.StdEncoding.DecodeString(signatureText)
	if err != nil {
		return err
	}
	digest := sha256.Sum256([]byte(alipaySignContent(params)))
	return rsa.VerifyPKCS1v15(runtime.publicKey, crypto.SHA256, digest[:], signature)
}

func (runtime *alipayRuntime) verifyGatewayResponse(response json.RawMessage, signatureText string) error {
	if len(response) == 0 {
		return errors.New("支付宝响应内容为空")
	}
	if signatureText == "" {
		return errors.New("支付宝响应签名为空")
	}
	signature, err := base64.StdEncoding.DecodeString(signatureText)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(response)
	return rsa.VerifyPKCS1v15(runtime.publicKey, crypto.SHA256, digest[:], signature)
}

func (runtime *alipayRuntime) gatewayParams(method string, bizContent any, notifyURL string) (url.Values, error) {
	bizBytes, err := common.Marshal(bizContent)
	if err != nil {
		return nil, err
	}
	params := map[string]string{
		"app_id":      runtime.config.AppID,
		"method":      method,
		"format":      "JSON",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
		"version":     "1.0",
		"biz_content": string(bizBytes),
	}
	if notifyURL != "" {
		params["notify_url"] = notifyURL
	}
	if runtime.config.AppAuthToken != "" {
		params["app_auth_token"] = runtime.config.AppAuthToken
	}
	if err = runtime.sign(params); err != nil {
		return nil, err
	}

	values := url.Values{}
	for key, value := range params {
		values.Set(key, value)
	}
	return values, nil
}

func (runtime *alipayRuntime) callGateway(ctx context.Context, params url.Values) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, runtime.config.GatewayURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=utf-8")

	response, err := runtime.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("支付宝网关返回 HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, alipayGatewayResponseLimit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > alipayGatewayResponseLimit {
		return nil, errors.New("支付宝响应体过大")
	}
	return body, nil
}

func alipayGatewayBusinessError(code, msg, subCode, subMsg string) error {
	if subCode == "ACQ.TRADE_NOT_EXIST" {
		return errAlipayTradeNotExist
	}
	message := subMsg
	if message == "" {
		message = msg
	}
	if message == "" {
		message = code
	}
	return errors.New(message)
}

func (runtime *alipayRuntime) precreate(ctx context.Context, bizContent alipayPrecreateBizContent) (alipayPrecreateResponse, error) {
	params, err := runtime.gatewayParams("alipay.trade.precreate", bizContent, runtime.config.NotifyURL)
	if err != nil {
		return alipayPrecreateResponse{}, err
	}
	body, err := runtime.callGateway(ctx, params)
	if err != nil {
		return alipayPrecreateResponse{}, err
	}
	var gatewayResponse alipayPrecreateGatewayResponse
	if err = common.Unmarshal(body, &gatewayResponse); err != nil {
		return alipayPrecreateResponse{}, err
	}
	if err = runtime.verifyGatewayResponse(gatewayResponse.Response, gatewayResponse.Sign); err != nil {
		return alipayPrecreateResponse{}, fmt.Errorf("支付宝预下单响应验签失败: %w", err)
	}
	var precreateResponse alipayPrecreateResponse
	if err = common.Unmarshal(gatewayResponse.Response, &precreateResponse); err != nil {
		return alipayPrecreateResponse{}, err
	}
	if precreateResponse.Code != "10000" {
		return alipayPrecreateResponse{}, alipayGatewayBusinessError(
			precreateResponse.Code,
			precreateResponse.Msg,
			precreateResponse.SubCode,
			precreateResponse.SubMsg,
		)
	}
	if precreateResponse.QRCode == "" {
		return alipayPrecreateResponse{}, errors.New("支付宝未返回二维码")
	}
	return precreateResponse, nil
}

func (runtime *alipayRuntime) query(ctx context.Context, tradeNo string) (alipayQueryResponse, error) {
	params, err := runtime.gatewayParams("alipay.trade.query", alipayQueryBizContent{OutTradeNo: tradeNo}, "")
	if err != nil {
		return alipayQueryResponse{}, err
	}
	body, err := runtime.callGateway(ctx, params)
	if err != nil {
		return alipayQueryResponse{}, err
	}
	var gatewayResponse alipayQueryGatewayResponse
	if err = common.Unmarshal(body, &gatewayResponse); err != nil {
		return alipayQueryResponse{}, err
	}
	if err = runtime.verifyGatewayResponse(gatewayResponse.Response, gatewayResponse.Sign); err != nil {
		return alipayQueryResponse{}, fmt.Errorf("支付宝查单响应验签失败: %w", err)
	}
	var queryResponse alipayQueryResponse
	if err = common.Unmarshal(gatewayResponse.Response, &queryResponse); err != nil {
		return alipayQueryResponse{}, err
	}
	if queryResponse.Code != "10000" {
		return alipayQueryResponse{}, alipayGatewayBusinessError(
			queryResponse.Code,
			queryResponse.Msg,
			queryResponse.SubCode,
			queryResponse.SubMsg,
		)
	}
	return queryResponse, nil
}

func (runtime *alipayRuntime) closeTrade(ctx context.Context, tradeNo string) error {
	params, err := runtime.gatewayParams("alipay.trade.close", alipayCloseBizContent{OutTradeNo: tradeNo}, "")
	if err != nil {
		return err
	}
	body, err := runtime.callGateway(ctx, params)
	if err != nil {
		return err
	}
	var gatewayResponse alipayCloseGatewayResponse
	if err = common.Unmarshal(body, &gatewayResponse); err != nil {
		return err
	}
	if err = runtime.verifyGatewayResponse(gatewayResponse.Response, gatewayResponse.Sign); err != nil {
		return fmt.Errorf("支付宝关单响应验签失败: %w", err)
	}
	var closeResponse alipayCloseResponse
	if err = common.Unmarshal(gatewayResponse.Response, &closeResponse); err != nil {
		return err
	}
	if closeResponse.Code != "10000" {
		return alipayGatewayBusinessError(
			closeResponse.Code,
			closeResponse.Msg,
			closeResponse.SubCode,
			closeResponse.SubMsg,
		)
	}
	if closeResponse.OutTradeNo != "" && closeResponse.OutTradeNo != tradeNo {
		return errors.New("支付宝关单响应订单号不匹配")
	}
	return nil
}

func alipaySuccessTime(raw string) time.Time {
	if raw == "" {
		return time.Time{}
	}
	successTime, err := time.ParseInLocation("2006-01-02 15:04:05", raw, time.Local)
	if err != nil {
		return time.Time{}
	}
	return successTime
}

func writeAlipayPrecreateResponse(c *gin.Context, order *model.AlipayOrder) {
	common.ApiSuccess(c, gin.H{
		"trade_no":   order.OutTradeNo,
		"qr_code":    order.QRCode,
		"amount_fen": order.AmountFen,
		"expires_at": order.ExpiresAt,
		"status":     order.Status,
	})
}

func RequestAlipayPrecreate(c *gin.Context) {
	if !isAlipayTopUpEnabled() {
		common.ApiErrorMsg(c, "支付宝扫码支付暂不可用")
		return
	}

	var req AlipayPrecreateRequest
	if err := c.ShouldBindJSON(&req); err != nil || !alipayClientRequestIDPattern.MatchString(req.ClientRequestID) {
		common.ApiErrorMsg(c, "参数错误")
		return
	}
	if req.Amount < getMinTopup() {
		common.ApiErrorMsg(c, fmt.Sprintf("充值数量不能小于 %d", getMinTopup()))
		return
	}

	userID := c.GetInt("id")
	existingOrder, err := model.GetAlipayOrderByClientRequest(userID, req.ClientRequestID)
	if err == nil {
		if existingOrder.Status != model.AlipayOrderStatusFailed && existingOrder.Status != model.AlipayOrderStatusClosed {
			writeAlipayPrecreateResponse(c, existingOrder)
			return
		}
		if err = model.ReleaseAlipayClientRequestForRetry(userID, req.ClientRequestID); err != nil {
			common.ApiErrorMsg(c, "重置支付宝订单失败")
			return
		}
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		common.ApiErrorMsg(c, "创建支付宝订单失败")
		return
	}

	group, err := model.GetUserGroup(userID, true)
	if err != nil {
		common.ApiErrorMsg(c, "获取用户分组失败")
		return
	}
	payMoney := getPayMoney(req.Amount, group)
	amountFen, err := alipayAmountFen(payMoney)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	normalizedAmount := normalizeAlipayTopUpAmount(req.Amount)
	if normalizedAmount <= 0 {
		common.ApiErrorMsg(c, "充值额度无效")
		return
	}
	creditQuota, err := alipayCreditQuota(req.Amount)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	runtime, err := getAlipayRuntime()
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("支付宝初始化失败 error=%q", err.Error()))
		common.ApiErrorMsg(c, "支付宝配置不可用")
		return
	}

	now := time.Now()
	expiresAt := now.Add(alipayOrderLifetime)
	tradeNo := fmt.Sprintf("AR%d%s", now.UnixMilli(), common.GetRandomString(8))
	topUp := &model.TopUp{
		UserId:          userID,
		Amount:          normalizedAmount,
		Money:           decimal.NewFromInt(amountFen).Div(decimal.NewFromInt(100)).InexactFloat64(),
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodAlipayPrecreate,
		PaymentProvider: model.PaymentProviderAlipayPrecreate,
		CreateTime:      now.Unix(),
		Status:          common.TopUpStatusPending,
	}
	order := &model.AlipayOrder{
		UserId:          userID,
		ClientRequestId: req.ClientRequestID,
		OutTradeNo:      tradeNo,
		AmountFen:       amountFen,
		CreditQuota:     creditQuota,
		Currency:        "CNY",
		Status:          model.AlipayOrderStatusPending,
		ExpiresAt:       expiresAt.Unix(),
		CreatedAt:       now.Unix(),
		UpdatedAt:       now.Unix(),
	}
	if err = model.CreateAlipayTopUp(topUp, order); err != nil {
		if duplicateOrder, lookupErr := model.GetAlipayOrderByClientRequest(userID, req.ClientRequestID); lookupErr == nil {
			writeAlipayPrecreateResponse(c, duplicateOrder)
			return
		}
		logger.LogError(c.Request.Context(), fmt.Sprintf("支付宝创建本地订单失败 user_id=%d error=%q", userID, err.Error()))
		common.ApiErrorMsg(c, "创建支付宝订单失败")
		return
	}

	description := fmt.Sprintf("BlackRain Relay 额度充值（%s元）", alipayAmountYuan(amountFen))
	response, err := runtime.precreate(c.Request.Context(), alipayPrecreateBizContent{
		OutTradeNo:     tradeNo,
		TotalAmount:    alipayAmountYuan(amountFen),
		Subject:        description,
		TimeoutExpress: "15m",
	})
	if err != nil {
		_ = model.MarkAlipayOrderFailed(tradeNo)
		logger.LogError(c.Request.Context(), fmt.Sprintf("支付宝预创建订单失败 user_id=%d trade_no=%s error=%q", userID, tradeNo, err.Error()))
		common.ApiErrorMsg(c, "支付宝下单失败")
		return
	}
	order.QRCode = response.QRCode
	if err = model.UpdateAlipayPrecreateResult(tradeNo, order.QRCode); err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("支付宝保存二维码失败 user_id=%d trade_no=%s error=%q", userID, tradeNo, err.Error()))
		common.ApiErrorMsg(c, "保存支付宝订单失败")
		return
	}
	writeAlipayPrecreateResponse(c, order)
}

func GetAlipayPrecreateStatus(c *gin.Context) {
	order, err := model.GetAlipayOrderForUser(c.GetInt("id"), c.Param("trade_no"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.Status(http.StatusNotFound)
			return
		}
		common.ApiErrorMsg(c, "查询支付宝订单失败")
		return
	}
	if order.Status == model.AlipayOrderStatusPending {
		claimed, claimErr := model.ClaimAlipayOrderCheck(order.OutTradeNo, 10*time.Second)
		if claimErr != nil {
			common.ApiErrorMsg(c, "查询支付宝订单失败")
			return
		}
		if claimed {
			reconcileAlipayOrder(c.Request.Context(), order)
			if refreshedOrder, refreshErr := model.GetAlipayOrderForUser(c.GetInt("id"), order.OutTradeNo); refreshErr == nil {
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

func reconcileAlipayOrder(ctx context.Context, order *model.AlipayOrder) {
	runtime, err := getAlipayRuntime()
	if err != nil {
		return
	}
	reconcileAlipayOrderWithRuntime(ctx, runtime, order)
}

func reconcileAlipayOrderWithRuntime(ctx context.Context, runtime *alipayRuntime, order *model.AlipayOrder) {
	queryResponse, err := runtime.query(ctx, order.OutTradeNo)
	if err != nil {
		if errors.Is(err, errAlipayTradeNotExist) && time.Now().Unix() >= order.ExpiresAt {
			if closeErr := model.CloseAlipayOrder(order.OutTradeNo); closeErr != nil {
				logger.LogError(ctx, fmt.Sprintf("支付宝关闭不存在订单失败 trade_no=%s error=%q", order.OutTradeNo, closeErr.Error()))
			}
			return
		}
		logger.LogWarn(ctx, fmt.Sprintf("支付宝主动查单失败 trade_no=%s error=%q", order.OutTradeNo, err.Error()))
		return
	}

	switch queryResponse.TradeStatus {
	case "TRADE_SUCCESS", "TRADE_FINISHED":
		amountFen, amountErr := alipayAmountFenFromYuan(queryResponse.TotalAmount)
		if amountErr != nil || queryResponse.OutTradeNo != order.OutTradeNo || queryResponse.TradeNo == "" {
			logger.LogWarn(ctx, fmt.Sprintf("支付宝主动查单字段不完整或金额无效 trade_no=%s", order.OutTradeNo))
			return
		}
		_, completionErr := model.CompleteAlipayTopUp(model.AlipayCompletion{
			EventID:     "query:" + queryResponse.TradeNo,
			OutTradeNo:  queryResponse.OutTradeNo,
			TradeNo:     queryResponse.TradeNo,
			AmountFen:   amountFen,
			Currency:    "CNY",
			SuccessTime: alipaySuccessTime(queryResponse.SendPayDate),
			BodyDigest:  fmt.Sprintf("%x", sha256.Sum256([]byte(queryResponse.TradeNo))),
		})
		if completionErr != nil {
			logger.LogError(ctx, fmt.Sprintf("支付宝主动查单入账失败 trade_no=%s error=%q", order.OutTradeNo, completionErr.Error()))
		}
	case "TRADE_CLOSED":
		if closeErr := model.CloseAlipayOrder(order.OutTradeNo); closeErr != nil {
			logger.LogError(ctx, fmt.Sprintf("支付宝同步远端关单状态失败 trade_no=%s error=%q", order.OutTradeNo, closeErr.Error()))
		}
	case "WAIT_BUYER_PAY":
		if time.Now().Unix() < order.ExpiresAt {
			return
		}
		if closeErr := runtime.closeTrade(ctx, order.OutTradeNo); closeErr != nil && !errors.Is(closeErr, errAlipayTradeNotExist) {
			logger.LogWarn(ctx, fmt.Sprintf("支付宝远程关单失败 trade_no=%s error=%q", order.OutTradeNo, closeErr.Error()))
			return
		}
		if closeErr := model.CloseAlipayOrder(order.OutTradeNo); closeErr != nil {
			logger.LogError(ctx, fmt.Sprintf("支付宝本地关单失败 trade_no=%s error=%q", order.OutTradeNo, closeErr.Error()))
		}
	}
}

func AlipayNotify(c *gin.Context) {
	if !isAlipayWebhookEnabled() {
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	runtime, err := getAlipayRuntime()
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("支付宝回调初始化失败 error=%q", err.Error()))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, alipayNotifyMaxBody)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	bodyDigest := fmt.Sprintf("%x", sha256.Sum256(body))
	formValues, err := url.ParseQuery(string(body))
	if err != nil {
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	params := map[string]string{}
	for key := range formValues {
		params[key] = formValues.Get(key)
	}
	if err = runtime.verify(params); err != nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("支付宝回调验签失败 body_digest=%s error=%q", bodyDigest, err.Error()))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	if params["app_id"] != runtime.config.AppID {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("支付宝回调应用身份不匹配 body_digest=%s", bodyDigest))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	tradeStatus := params["trade_status"]
	if tradeStatus != "TRADE_SUCCESS" && tradeStatus != "TRADE_FINISHED" {
		_, _ = c.Writer.Write([]byte("success"))
		return
	}
	amountFen, err := alipayAmountFenFromYuan(params["total_amount"])
	if err != nil || params["out_trade_no"] == "" || params["trade_no"] == "" {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("支付宝回调字段不完整 body_digest=%s", bodyDigest))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	eventID := params["notify_id"]
	if eventID == "" {
		eventID = "notify:" + bodyDigest
	}

	_, err = model.CompleteAlipayTopUp(model.AlipayCompletion{
		EventID:     eventID,
		OutTradeNo:  params["out_trade_no"],
		TradeNo:     params["trade_no"],
		AmountFen:   amountFen,
		Currency:    "CNY",
		SuccessTime: alipaySuccessTime(params["gmt_payment"]),
		BodyDigest:  bodyDigest,
	})
	if err != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("支付宝回调入账失败 trade_no=%s error=%q", params["out_trade_no"], err.Error()))
		_, _ = c.Writer.Write([]byte("fail"))
		return
	}
	_, _ = c.Writer.Write([]byte("success"))
}
