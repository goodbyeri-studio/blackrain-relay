package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitChannelCacheHandlesEnabledChannelWithoutAbilitySnapshot(t *testing.T) {
	resetPricingEndpointTestTables(t)

	baseURL := "https://example.test"
	channel := &Channel{
		Id:      98001,
		Type:    constant.ChannelTypeOpenAI,
		Key:     "test-key",
		Status:  common.ChannelStatusEnabled,
		Name:    "cache-race-channel",
		Group:   "cache-race-group",
		Models:  "cache-race-model",
		BaseURL: &baseURL,
	}
	require.NoError(t, DB.Create(channel).Error)

	require.NotPanics(t, InitChannelCache)
	cached, err := GetRandomSatisfiedChannel("cache-race-group", "cache-race-model", 0, "/v1/chat/completions")
	require.NoError(t, err)
	require.NotNil(t, cached)
	assert.Equal(t, channel.Id, cached.Id)
}
