package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterPricingByUsableGroups(t *testing.T) {
	pricing := []model.Pricing{
		{ModelName: "allowed", EnableGroup: []string{"deepkey"}},
		{ModelName: "blocked", EnableGroup: []string{"private"}},
		{ModelName: "global", EnableGroup: []string{"all"}},
	}

	filtered := filterPricingByUsableGroups(pricing, map[string]string{"deepkey": "DeepKey"})

	require.Len(t, filtered, 2)
	assert.Equal(t, "allowed", filtered[0].ModelName)
	assert.Equal(t, "global", filtered[1].ModelName)
}
