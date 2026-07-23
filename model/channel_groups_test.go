package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetEnabledChannelGroups(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.Create(&[]Channel{
		{Name: "enabled-one", Key: "key-one", Status: common.ChannelStatusEnabled, Group: "alpha,beta"},
		{Name: "enabled-two", Key: "key-two", Status: common.ChannelStatusEnabled, Group: " beta, gamma "},
		{Name: "disabled", Key: "key-three", Status: common.ChannelStatusManuallyDisabled, Group: "disabled"},
	}).Error)

	groups, err := GetEnabledChannelGroups()
	require.NoError(t, err)
	assert.Equal(t, map[string]struct{}{
		"alpha": {},
		"beta":  {},
		"gamma": {},
	}, groups)
}
