package controller

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
