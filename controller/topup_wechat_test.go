package controller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
