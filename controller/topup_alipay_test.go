package controller

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func alipayGatewayTestBody(t *testing.T, privateKey *rsa.PrivateKey, field string, response, signedResponse json.RawMessage, includeSign bool) []byte {
	t.Helper()
	digest := sha256.Sum256(signedResponse)
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	require.NoError(t, err)
	payload := map[string]any{field: response}
	if includeSign {
		payload["sign"] = base64.StdEncoding.EncodeToString(signature)
	}
	body, err := common.Marshal(payload)
	require.NoError(t, err)
	return body
}

func newAlipayTestRuntime(privateKey *rsa.PrivateKey, gatewayURL string, httpClient *http.Client) *alipayRuntime {
	return &alipayRuntime{
		config: setting.AlipayConfig{
			AppID:      "2021000000000000",
			NotifyURL:  "https://relay.example.com/api/alipay/notify",
			GatewayURL: gatewayURL,
		},
		privateKey: privateKey,
		publicKey:  &privateKey.PublicKey,
		httpClient: httpClient,
	}
}

func createAlipayOrderForControllerTest(t *testing.T, tradeNo string, expiresAt int64) *model.AlipayOrder {
	t.Helper()
	topUp := &model.TopUp{
		UserId:          801,
		Amount:          10,
		Money:           10,
		TradeNo:         tradeNo,
		PaymentMethod:   model.PaymentMethodAlipayPrecreate,
		PaymentProvider: model.PaymentProviderAlipayPrecreate,
		CreateTime:      time.Now().Unix(),
		Status:          common.TopUpStatusPending,
	}
	order := &model.AlipayOrder{
		UserId:          801,
		ClientRequestId: "controller_test_123456",
		OutTradeNo:      tradeNo,
		AmountFen:       1000,
		CreditQuota:     10,
		Currency:        "CNY",
		Status:          model.AlipayOrderStatusPending,
		ExpiresAt:       expiresAt,
		CreatedAt:       time.Now().Unix(),
		UpdatedAt:       time.Now().Unix(),
	}
	require.NoError(t, model.CreateAlipayTopUp(topUp, order))
	return order
}

func TestAlipayAmountFenRoundsToNearestFen(t *testing.T) {
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
			actual, err := alipayAmountFen(testCase.payMoney)
			if testCase.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, testCase.expected, actual)
		})
	}
}

func TestAlipayGatewayResponseVerificationUsesOriginalResponseNode(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	runtime := &alipayRuntime{publicKey: &privateKey.PublicKey}
	response := json.RawMessage("{\"code\":\"10000\",\"trade_status\":\"TRADE_SUCCESS\",\"total_amount\":\"50.00\"}")
	digest := sha256.Sum256(response)
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	require.NoError(t, err)
	signatureText := base64.StdEncoding.EncodeToString(signature)

	require.NoError(t, runtime.verifyGatewayResponse(response, signatureText))

	tamperedResponse := json.RawMessage("{\"code\":\"10000\",\"trade_status\":\"TRADE_SUCCESS\",\"total_amount\":\"500.00\"}")
	require.Error(t, runtime.verifyGatewayResponse(tamperedResponse, signatureText))
	require.Error(t, runtime.verifyGatewayResponse(response, ""))
}

func TestAlipayPrecreateAndQueryValidateGatewayHTTPResponses(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	methods := []struct {
		name          string
		gatewayMethod string
		field         string
		validResponse json.RawMessage
		tampered      json.RawMessage
		invoke        func(*alipayRuntime) error
	}{
		{
			name:          "precreate",
			gatewayMethod: "alipay.trade.precreate",
			field:         "alipay_trade_precreate_response",
			validResponse: json.RawMessage(`{"code":"10000","out_trade_no":"T1","qr_code":"https://qr.example.com/T1"}`),
			tampered:      json.RawMessage(`{"code":"10000","out_trade_no":"T1","qr_code":"https://attacker.example.com/T1"}`),
			invoke: func(runtime *alipayRuntime) error {
				_, callErr := runtime.precreate(context.Background(), alipayPrecreateBizContent{OutTradeNo: "T1", TotalAmount: "1.00", Subject: "test"})
				return callErr
			},
		},
		{
			name:          "query",
			gatewayMethod: "alipay.trade.query",
			field:         "alipay_trade_query_response",
			validResponse: json.RawMessage(`{"code":"10000","out_trade_no":"T1","trade_no":"A1","trade_status":"WAIT_BUYER_PAY","total_amount":"1.00"}`),
			tampered:      json.RawMessage(`{"code":"10000","out_trade_no":"T1","trade_no":"A1","trade_status":"TRADE_SUCCESS","total_amount":"100.00"}`),
			invoke: func(runtime *alipayRuntime) error {
				_, callErr := runtime.query(context.Background(), "T1")
				return callErr
			},
		},
	}

	for _, method := range methods {
		method := method
		t.Run(method.name, func(t *testing.T) {
			testCases := []struct {
				name       string
				statusCode int
				body       []byte
				wantError  bool
			}{
				{
					name:       "valid signature",
					statusCode: http.StatusOK,
					body:       alipayGatewayTestBody(t, privateKey, method.field, method.validResponse, method.validResponse, true),
				},
				{
					name:       "tampered response",
					statusCode: http.StatusOK,
					body:       alipayGatewayTestBody(t, privateKey, method.field, method.tampered, method.validResponse, true),
					wantError:  true,
				},
				{
					name:       "missing signature",
					statusCode: http.StatusOK,
					body:       alipayGatewayTestBody(t, privateKey, method.field, method.validResponse, method.validResponse, false),
					wantError:  true,
				},
				{name: "http error", statusCode: http.StatusInternalServerError, body: []byte("failed"), wantError: true},
				{name: "oversized response", statusCode: http.StatusOK, body: []byte(strings.Repeat("x", int(alipayGatewayResponseLimit)+1)), wantError: true},
			}

			for _, testCase := range testCases {
				t.Run(testCase.name, func(t *testing.T) {
					receivedMethod := make(chan string, 1)
					server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
						receivedMethod <- request.FormValue("method")
						writer.WriteHeader(testCase.statusCode)
						_, _ = writer.Write(testCase.body)
					}))
					defer server.Close()

					runtime := newAlipayTestRuntime(privateKey, server.URL, server.Client())
					callErr := method.invoke(runtime)
					if testCase.wantError {
						require.Error(t, callErr)
					} else {
						require.NoError(t, callErr)
					}
					require.Equal(t, method.gatewayMethod, <-receivedMethod)
				})
			}
		})
	}
}

func TestReconcileAlipayOrderOnlyClosesAfterRemoteConfirmation(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	testCases := []struct {
		name           string
		expiresAt      int64
		closeStatus    int
		closeNotExist  bool
		expectedOrder  string
		expectedTopUp  string
		expectedCloses int
	}{
		{
			name:           "expired and remote close succeeds",
			expiresAt:      time.Now().Add(-time.Minute).Unix(),
			closeStatus:    http.StatusOK,
			expectedOrder:  model.AlipayOrderStatusClosed,
			expectedTopUp:  common.TopUpStatusExpired,
			expectedCloses: 1,
		},
		{
			name:           "expired and remote reports missing trade",
			expiresAt:      time.Now().Add(-time.Minute).Unix(),
			closeStatus:    http.StatusOK,
			closeNotExist:  true,
			expectedOrder:  model.AlipayOrderStatusClosed,
			expectedTopUp:  common.TopUpStatusExpired,
			expectedCloses: 1,
		},
		{
			name:           "remote close failure keeps pending",
			expiresAt:      time.Now().Add(-time.Minute).Unix(),
			closeStatus:    http.StatusInternalServerError,
			expectedOrder:  model.AlipayOrderStatusPending,
			expectedTopUp:  common.TopUpStatusPending,
			expectedCloses: 1,
		},
		{
			name:           "unexpired order is not closed",
			expiresAt:      time.Now().Add(time.Minute).Unix(),
			closeStatus:    http.StatusOK,
			expectedOrder:  model.AlipayOrderStatusPending,
			expectedTopUp:  common.TopUpStatusPending,
			expectedCloses: 0,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			db := setupModelListControllerTestDB(t)
			require.NoError(t, db.AutoMigrate(&model.TopUp{}, &model.AlipayOrder{}, &model.AlipayNotification{}))
			tradeNo := "close-" + strings.ReplaceAll(testCase.name, " ", "-")
			order := createAlipayOrderForControllerTest(t, tradeNo, testCase.expiresAt)

			queryResponse := json.RawMessage(`{"code":"10000","out_trade_no":"` + tradeNo + `","trade_status":"WAIT_BUYER_PAY"}`)
			closeResponse := json.RawMessage(`{"code":"10000","out_trade_no":"` + tradeNo + `"}`)
			if testCase.closeNotExist {
				closeResponse = json.RawMessage(`{"code":"40004","sub_code":"ACQ.TRADE_NOT_EXIST","sub_msg":"Trade does not exist"}`)
			}
			queryBody := alipayGatewayTestBody(t, privateKey, "alipay_trade_query_response", queryResponse, queryResponse, true)
			closeBody := alipayGatewayTestBody(t, privateKey, "alipay_trade_close_response", closeResponse, closeResponse, true)
			var closeCalls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				switch request.FormValue("method") {
				case "alipay.trade.query":
					_, _ = writer.Write(queryBody)
				case "alipay.trade.close":
					closeCalls.Add(1)
					writer.WriteHeader(testCase.closeStatus)
					_, _ = writer.Write(closeBody)
				default:
					writer.WriteHeader(http.StatusBadRequest)
				}
			}))
			defer server.Close()

			runtime := newAlipayTestRuntime(privateKey, server.URL, server.Client())
			reconcileAlipayOrderWithRuntime(context.Background(), runtime, order)

			var storedOrder model.AlipayOrder
			require.NoError(t, db.Where("out_trade_no = ?", tradeNo).First(&storedOrder).Error)
			var storedTopUp model.TopUp
			require.NoError(t, db.Where("trade_no = ?", tradeNo).First(&storedTopUp).Error)
			assert.Equal(t, testCase.expectedOrder, storedOrder.Status)
			assert.Equal(t, testCase.expectedTopUp, storedTopUp.Status)
			assert.Equal(t, int32(testCase.expectedCloses), closeCalls.Load())

			if testCase.expectedOrder == model.AlipayOrderStatusClosed {
				credited, completionErr := model.CompleteAlipayTopUp(model.AlipayCompletion{
					EventID:     "late-" + tradeNo,
					OutTradeNo:  tradeNo,
					TradeNo:     "late-provider-trade",
					AmountFen:   storedOrder.AmountFen,
					Currency:    "CNY",
					SuccessTime: time.Now(),
					BodyDigest:  "late-digest",
				})
				require.Error(t, completionErr)
				assert.False(t, credited)
			}
		})
	}
}

func TestReconcileAlipayOrderQueryTimeoutKeepsPending(t *testing.T) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.TopUp{}, &model.AlipayOrder{}, &model.AlipayNotification{}))
	tradeNo := "query-timeout"
	order := createAlipayOrderForControllerTest(t, tradeNo, time.Now().Add(-time.Minute).Unix())

	releaseRequest := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		<-releaseRequest
	}))
	defer func() {
		close(releaseRequest)
		server.Close()
	}()
	runtime := newAlipayTestRuntime(privateKey, server.URL, server.Client())
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	reconcileAlipayOrderWithRuntime(ctx, runtime, order)

	var storedOrder model.AlipayOrder
	require.NoError(t, db.Where("out_trade_no = ?", tradeNo).First(&storedOrder).Error)
	var storedTopUp model.TopUp
	require.NoError(t, db.Where("trade_no = ?", tradeNo).First(&storedTopUp).Error)
	assert.Equal(t, model.AlipayOrderStatusPending, storedOrder.Status)
	assert.Equal(t, common.TopUpStatusPending, storedTopUp.Status)
}

func TestAlipaySignContentSortsAndSkipsSignatureFields(t *testing.T) {
	params := map[string]string{
		"sign":        "ignored",
		"sign_type":   "RSA2",
		"method":      "alipay.trade.precreate",
		"app_id":      "2021000000000000",
		"biz_content": `{"out_trade_no":"T1"}`,
	}

	assert.Equal(t, `app_id=2021000000000000&biz_content={"out_trade_no":"T1"}&method=alipay.trade.precreate`, alipaySignContent(params))
}
