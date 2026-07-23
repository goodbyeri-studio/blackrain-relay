package service

import (
	"errors"
	"net/http"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type failingBillingSettler struct {
	err error
}

func (s *failingBillingSettler) Settle(int) error         { return nil }
func (s *failingBillingSettler) Refund(*gin.Context)      {}
func (s *failingBillingSettler) NeedsRefund() bool        { return false }
func (s *failingBillingSettler) GetPreConsumedQuota() int { return 0 }
func (s *failingBillingSettler) Reserve(int) error        { return s.err }

func TestReserveBillingPreservesStructuredBillingError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)
	expected := types.NewErrorWithStatusCode(
		errors.New("token quota reserve failed"),
		types.ErrorCodePreConsumeTokenQuotaFailed,
		http.StatusForbidden,
		types.ErrOptionWithSkipRetry(),
	)
	info := &relaycommon.RelayInfo{
		Billing: &failingBillingSettler{err: expected},
	}

	actual := ReserveBilling(ctx, 100, info)

	require.Same(t, expected, actual)
}

func TestReserveBillingRejectsNilRelayInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)

	apiErr := ReserveBilling(ctx, 100, nil)

	require.NotNil(t, apiErr)
	require.Equal(t, types.ErrorCodeInvalidRequest, apiErr.GetErrorCode())
}
