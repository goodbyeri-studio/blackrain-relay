package controller

import (
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
