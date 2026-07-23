package controller

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type recordingBillingSettler struct {
	reserveTargets []int
	reserveErr     error
}

func (s *recordingBillingSettler) Settle(int) error         { return nil }
func (s *recordingBillingSettler) Refund(*gin.Context)      {}
func (s *recordingBillingSettler) NeedsRefund() bool        { return false }
func (s *recordingBillingSettler) GetPreConsumedQuota() int { return 1000 }
func (s *recordingBillingSettler) Reserve(targetQuota int) error {
	s.reserveTargets = append(s.reserveTargets, targetQuota)
	return s.reserveErr
}

func TestRefreshRelayPriceForSelectedChannelReservesChannelQuota(t *testing.T) {
	gin.SetMode(gin.TestMode)
	savedModelPrices := ratio_setting.ModelPrice2JSONString()
	savedModelRatios := ratio_setting.ModelRatio2JSONString()
	t.Cleanup(func() {
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(savedModelPrices))
		require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(savedModelRatios))
	})

	modelPrices, err := common.Marshal(map[string]float64{})
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(string(modelPrices)))
	modelRatios, err := common.Marshal(map[string]float64{"retry-channel-model": 1})
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(string(modelRatios)))

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Set("group", "default")
	common.SetContextKey(ctx, constant.ContextKeyChannelOtherSetting, dto.ChannelOtherSettings{
		ModelRatios: map[string]float64{"retry-channel-model": 2.5},
	})
	billing := &recordingBillingSettler{}
	info := &relaycommon.RelayInfo{
		OriginModelName: "retry-channel-model",
		UserGroup:       "default",
		UsingGroup:      "default",
		Billing:         billing,
	}

	apiErr := refreshRelayPriceForSelectedChannel(ctx, info, 1000, &types.TokenCountMeta{})

	require.Nil(t, apiErr)
	require.Equal(t, 2.5, info.PriceData.ModelRatio)
	require.Equal(t, 2500, info.PriceData.QuotaToPreConsume)
	require.Equal(t, []int{2500}, billing.reserveTargets)
}
