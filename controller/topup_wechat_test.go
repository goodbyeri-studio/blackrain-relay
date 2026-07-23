package controller

import (
	"bytes"
	"context"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wechatpay-apiv3/wechatpay-go/core"
	"github.com/wechatpay-apiv3/wechatpay-go/core/auth/verifiers"
	"github.com/wechatpay-apiv3/wechatpay-go/core/notify"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments"
	"github.com/wechatpay-apiv3/wechatpay-go/services/payments/native"
	"gorm.io/gorm"
)

type fakeWechatNativeService struct {
	prepayResponse *native.PrepayResponse
	prepayErr      error
	transaction    *payments.Transaction
	queryErr       error
	closeErr       error
	closeCalls     int
}

func (f *fakeWechatNativeService) Prepay(context.Context, native.PrepayRequest) (*native.PrepayResponse, *core.APIResult, error) {
	return f.prepayResponse, nil, f.prepayErr
}

func (f *fakeWechatNativeService) QueryOrderByOutTradeNo(context.Context, native.QueryOrderByOutTradeNoRequest) (*payments.Transaction, *core.APIResult, error) {
	return f.transaction, nil, f.queryErr
}

func (f *fakeWechatNativeService) CloseOrder(context.Context, native.CloseOrderRequest) (*core.APIResult, error) {
	f.closeCalls++
	return nil, f.closeErr
}

type fakeWechatNotifyHandler struct {
	request     *notify.Request
	transaction *payments.Transaction
	err         error
}

func (f *fakeWechatNotifyHandler) ParseNotifyRequest(_ context.Context, _ *http.Request, content interface{}) (*notify.Request, error) {
	if f.transaction != nil {
		*content.(*payments.Transaction) = *f.transaction
	}
	return f.request, f.err
}

func newWechatControllerRuntime(nativeService wechatNativeService) *wechatPayRuntime {
	return &wechatPayRuntime{
		config: setting.WechatPayConfig{
			AppID:     "wx-test-app",
			MchID:     "test-mch",
			NotifyURL: "https://relay.example.com/api/payment/wechat/notify",
		},
		nativeService: nativeService,
	}
}

func TestWechatPayAmountFenRoundsToNearestFen(t *testing.T) {
	testCases := []struct {
		name      string
		payMoney  float64
		expected  int64
		wantError bool
	}{
		{name: "whole yuan", payMoney: 10, expected: 1000},
		{name: "fractional yuan", payMoney: 19.995, expected: 2000},
		{name: "below one fen", payMoney: 0.001, wantError: true},
		{name: "above maximum", payMoney: 1_000_000.01, wantError: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actual, err := wechatPayAmountFen(testCase.payMoney)
			if testCase.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, testCase.expected, actual)
		})
	}
}

func TestPrepayWechatNativeOrderValidatesProtocolResponse(t *testing.T) {
	expiresAt := time.Now().Add(15 * time.Minute)
	service := &fakeWechatNativeService{prepayResponse: &native.PrepayResponse{CodeUrl: core.String("weixin://wxpay/bizpayurl?pr=test")}}
	codeURL, err := prepayWechatNativeOrder(context.Background(), newWechatControllerRuntime(service), "WR-test", 100, "test", expiresAt)
	require.NoError(t, err)
	assert.Equal(t, "weixin://wxpay/bizpayurl?pr=test", codeURL)

	for _, testCase := range []struct {
		name string
		resp *native.PrepayResponse
		err  error
	}{
		{name: "http or signature failure", err: errors.New("signature invalid")},
		{name: "empty response", resp: &native.PrepayResponse{}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			service := &fakeWechatNativeService{prepayResponse: testCase.resp, prepayErr: testCase.err}
			_, err := prepayWechatNativeOrder(context.Background(), newWechatControllerRuntime(service), "WR-test", 100, "test", expiresAt)
			require.Error(t, err)
		})
	}
}

func TestReconcileWechatPayOrderStateTransitions(t *testing.T) {
	testCases := []struct {
		name          string
		status        string
		expiresAt     int64
		transaction   *payments.Transaction
		queryErr      error
		closeErr      error
		wantOrder     string
		wantTopUp     string
		wantCloseCall int
	}{
		{
			name:        "not pay before expiry stays pending",
			status:      "NOTPAY",
			expiresAt:   time.Now().Add(time.Minute).Unix(),
			transaction: &payments.Transaction{TradeState: core.String("NOTPAY")},
			wantOrder:   model.WechatPayOrderStatusPending, wantTopUp: common.TopUpStatusPending,
		},
		{
			name:      "query failure stays pending",
			expiresAt: time.Now().Add(-time.Minute).Unix(),
			queryErr:  errors.New("timeout"),
			wantOrder: model.WechatPayOrderStatusPending, wantTopUp: common.TopUpStatusPending,
		},
		{
			name:   "expired not pay close failure stays pending",
			status: "NOTPAY", expiresAt: time.Now().Add(-time.Minute).Unix(),
			transaction: &payments.Transaction{TradeState: core.String("NOTPAY")}, closeErr: errors.New("remote unavailable"),
			wantOrder: model.WechatPayOrderStatusPending, wantTopUp: common.TopUpStatusPending, wantCloseCall: 1,
		},
		{
			name:   "expired not pay close success closes locally",
			status: "NOTPAY", expiresAt: time.Now().Add(-time.Minute).Unix(),
			transaction: &payments.Transaction{TradeState: core.String("NOTPAY")},
			wantOrder:   model.WechatPayOrderStatusClosed, wantTopUp: common.TopUpStatusExpired, wantCloseCall: 1,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			db := setupModelListControllerTestDB(t)
			require.NoError(t, db.AutoMigrate(&model.User{}, &model.TopUp{}, &model.WechatPayOrder{}, &model.WechatPayNotification{}))
			tradeNo := "wechat-controller-" + testCase.name
			topUp := &model.TopUp{UserId: 901, Amount: 2, Money: 1, TradeNo: tradeNo, PaymentMethod: model.PaymentMethodWechatNative, PaymentProvider: model.PaymentProviderWechatNative, CreateTime: time.Now().Unix(), Status: common.TopUpStatusPending}
			order := &model.WechatPayOrder{UserId: 901, ClientRequestId: "wechat_controller_request", OutTradeNo: tradeNo, AmountFen: 100, CreditQuota: 2, Currency: "CNY", Status: model.WechatPayOrderStatusPending, ExpiresAt: testCase.expiresAt, CreatedAt: time.Now().Unix(), UpdatedAt: time.Now().Unix()}
			require.NoError(t, model.CreateWechatPayTopUp(topUp, order))
			service := &fakeWechatNativeService{transaction: testCase.transaction, queryErr: testCase.queryErr, closeErr: testCase.closeErr}
			if testCase.transaction != nil && testCase.transaction.TradeState != nil && *testCase.transaction.TradeState == "NOTPAY" {
				service.transaction.OutTradeNo = core.String(tradeNo)
			}
			runtime := newWechatControllerRuntime(service)
			reconcileWechatPayOrderWithRuntime(context.Background(), order, runtime)
			var storedOrder model.WechatPayOrder
			require.NoError(t, db.Where("out_trade_no = ?", tradeNo).First(&storedOrder).Error)
			var storedTopUp model.TopUp
			require.NoError(t, db.Where("trade_no = ?", tradeNo).First(&storedTopUp).Error)
			assert.Equal(t, testCase.wantOrder, storedOrder.Status)
			assert.Equal(t, testCase.wantTopUp, storedTopUp.Status)
			assert.Equal(t, testCase.wantCloseCall, service.closeCalls)
		})
	}
}

func buildWechatNotifyRequest(t *testing.T, platformKey *rsa.PrivateKey, apiV3Key, appID string) *http.Request {
	t.Helper()
	nonce := "0123456789ab"
	additionalData := "transaction"
	transaction, err := common.Marshal(map[string]interface{}{
		"appid":          appID,
		"mchid":          "test-mch",
		"out_trade_no":   "wechat-notify-test",
		"transaction_id": "wechat-notify-transaction",
		"trade_state":    "SUCCESS",
		"amount":         map[string]interface{}{"total": 100, "currency": "CNY"},
		"success_time":   time.Now().UTC().Format(time.RFC3339),
	})
	require.NoError(t, err)
	block, err := aes.NewCipher([]byte(apiV3Key))
	require.NoError(t, err)
	aead, err := cipher.NewGCM(block)
	require.NoError(t, err)
	ciphertext := aead.Seal(nil, []byte(nonce), transaction, []byte(additionalData))
	body, err := common.Marshal(map[string]interface{}{
		"id":         "wechat-notify-event",
		"event_type": "TRANSACTION.SUCCESS",
		"resource": map[string]string{
			"algorithm":       "AEAD_AES_256_GCM",
			"ciphertext":      base64.StdEncoding.EncodeToString(ciphertext),
			"associated_data": additionalData,
			"nonce":           nonce,
		},
	})
	require.NoError(t, err)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	signatureNonce := "signature-nonce"
	message := timestamp + "\n" + signatureNonce + "\n" + string(body) + "\n"
	digest := sha256.Sum256([]byte(message))
	signature, err := rsa.SignPKCS1v15(rand.Reader, platformKey, crypto.SHA256, digest[:])
	require.NoError(t, err)
	request := httptest.NewRequest(http.MethodPost, "/api/payment/wechat/notify", bytes.NewReader(body))
	request.Header.Set("Wechatpay-Signature-Type", "WECHATPAY2-SHA256-RSA2048")
	request.Header.Set("Wechatpay-Serial", "wechat-public-key")
	request.Header.Set("Wechatpay-Timestamp", timestamp)
	request.Header.Set("Wechatpay-Nonce", signatureNonce)
	request.Header.Set("Wechatpay-Signature", base64.StdEncoding.EncodeToString(signature))
	return request
}

func TestWechatPayNotifyVerifiesDecryptsAndRejectsTampering(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}, &model.User{}, &model.TopUp{}, &model.WechatPayOrder{}, &model.WechatPayNotification{}))
	topUp := &model.TopUp{
		UserId:          901,
		Amount:          2,
		Money:           1,
		TradeNo:         "wechat-notify-test",
		PaymentMethod:   model.PaymentMethodWechatNative,
		PaymentProvider: model.PaymentProviderWechatNative,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	order := &model.WechatPayOrder{
		UserId:          901,
		ClientRequestId: "wechat_notify_request",
		OutTradeNo:      "wechat-notify-test",
		AmountFen:       100,
		CreditQuota:     2,
		Currency:        "CNY",
		Status:          model.WechatPayOrderStatusPending,
		ExpiresAt:       time.Now().Add(time.Minute).Unix(),
		CreatedAt:       time.Now().Unix(),
		UpdatedAt:       time.Now().Unix(),
	}
	require.NoError(t, db.Create(&model.User{Id: 901, Username: "wechat-notify-user", Status: common.UserStatusEnabled}).Error)
	require.NoError(t, model.CreateWechatPayTopUp(topUp, order))

	platformKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	apiV3Key := "01234567890123456789012345678901"
	verifier := verifiers.NewSHA256WithRSAPubkeyVerifier("wechat-public-key", platformKey.PublicKey)
	notifyHandler, err := notify.NewRSANotifyHandler(apiV3Key, verifier)
	require.NoError(t, err)
	secretDir := t.TempDir()
	privateKeyPath := filepath.Join(secretDir, "merchant.pem")
	publicKeyPath := filepath.Join(secretDir, "wechatpay.pem")
	require.NoError(t, os.WriteFile(privateKeyPath, []byte("test"), 0o600))
	require.NoError(t, os.WriteFile(publicKeyPath, []byte("test"), 0o600))
	t.Setenv("WECHAT_PAY_APP_ID", "wx-test-app")
	t.Setenv("WECHAT_PAY_MCH_ID", "test-mch")
	t.Setenv("WECHAT_PAY_MERCHANT_SERIAL_NO", "merchant-serial")
	t.Setenv("WECHAT_PAY_MERCHANT_PRIVATE_KEY_PATH", privateKeyPath)
	t.Setenv("WECHAT_PAY_PUBLIC_KEY_ID", "wechat-public-key")
	t.Setenv("WECHAT_PAY_PUBLIC_KEY_PATH", publicKeyPath)
	t.Setenv("WECHAT_PAY_API_V3_KEY", apiV3Key)
	t.Setenv("WECHAT_PAY_NOTIFY_URL", "https://relay.example.com/api/payment/wechat/notify")
	runtime := &wechatPayRuntime{
		config:        setting.WechatPayConfig{AppID: "wx-test-app", MchID: "test-mch", PublicKeyID: "wechat-public-key", APIv3Key: apiV3Key},
		nativeService: &fakeWechatNativeService{},
		notifyHandler: notifyHandler,
	}
	originalWebhookEnabled := wechatPayWebhookEnabled
	wechatPayWebhookEnabled = func() bool { return true }
	originalLoader := wechatPayRuntimeLoader
	wechatPayRuntimeLoader = func(context.Context) (*wechatPayRuntime, error) { return runtime, nil }
	t.Cleanup(func() {
		wechatPayWebhookEnabled = originalWebhookEnabled
		wechatPayRuntimeLoader = originalLoader
	})

	gin.SetMode(gin.TestMode)
	callNotify := func(request *http.Request) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		context, _ := gin.CreateTestContext(recorder)
		context.Request = request
		WechatPayNotify(context)
		return recorder
	}

	valid := callNotify(buildWechatNotifyRequest(t, platformKey, apiV3Key, "wx-test-app"))
	assert.Equal(t, http.StatusNoContent, valid.Code)
	validAgain := callNotify(buildWechatNotifyRequest(t, platformKey, apiV3Key, "wx-test-app"))
	assert.Equal(t, http.StatusNoContent, validAgain.Code)

	var storedOrder model.WechatPayOrder
	require.NoError(t, db.Where("out_trade_no = ?", order.OutTradeNo).First(&storedOrder).Error)
	assert.Equal(t, model.WechatPayOrderStatusCredited, storedOrder.Status)
	assert.Equal(t, 2, getWechatNotifyUserQuota(t, db, 901))

	tamperedRequest := buildWechatNotifyRequest(t, platformKey, apiV3Key, "wx-test-app")
	tamperedRequest.Header.Set("Wechatpay-Signature", base64.StdEncoding.EncodeToString([]byte("tampered")))
	tampered := callNotify(tamperedRequest)
	assert.Equal(t, http.StatusUnauthorized, tampered.Code)
}

func getWechatNotifyUserQuota(t *testing.T, db *gorm.DB, userID int) int {
	t.Helper()
	var user model.User
	require.NoError(t, db.Select("quota").Where("id = ?", userID).First(&user).Error)
	return user.Quota
}

func TestWechatPayNotifyRejectsParserErrorsAndIdentityMismatch(t *testing.T) {
	previousEnabled := wechatPayWebhookEnabled
	previousLoader := wechatPayRuntimeLoader
	t.Cleanup(func() {
		wechatPayWebhookEnabled = previousEnabled
		wechatPayRuntimeLoader = previousLoader
	})
	wechatPayWebhookEnabled = func() bool { return true }

	for _, testCase := range []struct {
		name       string
		handler    *fakeWechatNotifyHandler
		wantStatus int
	}{
		{name: "tampered signature or decrypt failure", handler: &fakeWechatNotifyHandler{err: errors.New("invalid notification")}, wantStatus: http.StatusUnauthorized},
		{name: "wrong merchant identity", handler: &fakeWechatNotifyHandler{
			request: &notify.Request{ID: "event-identity"},
			transaction: &payments.Transaction{
				TradeState: core.String("SUCCESS"), OutTradeNo: core.String("missing-order"), TransactionId: core.String("tx"),
				Amount: &payments.TransactionAmount{Total: core.Int64(100), Currency: core.String("CNY")}, Appid: core.String("wrong-app"), Mchid: core.String("test-mch"), SuccessTime: core.String(time.Now().Format(time.RFC3339)),
			},
		}, wantStatus: http.StatusBadRequest},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			wechatPayRuntimeLoader = func(context.Context) (*wechatPayRuntime, error) {
				return &wechatPayRuntime{config: setting.WechatPayConfig{AppID: "wx-test-app", MchID: "test-mch", APIv3Key: "test"}, notifyHandler: testCase.handler}, nil
			}
			recorder := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/api/payment/wechat/notify", strings.NewReader(`{"resource":{}}`))
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = request
			WechatPayNotify(ctx)
			assert.Equal(t, testCase.wantStatus, recorder.Code)
		})
	}
}
