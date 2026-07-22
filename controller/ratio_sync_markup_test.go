package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyPricingMarkupOnlyChangesBasePrices(t *testing.T) {
	data := map[string]any{
		"model_ratio":      map[string]any{"token-model": 2.5, "free-model": 0.0},
		"completion_ratio": map[string]any{"token-model": 5.0},
		"cache_ratio":      map[string]any{"token-model": 0.1},
		"model_price":      map[string]any{"image-model": 0.8},
	}

	applyPricingMarkup(data, 30)

	assert.Equal(t, map[string]any{"token-model": 3.25, "free-model": 0.0}, data["model_ratio"])
	assert.Equal(t, map[string]any{"image-model": 1.04}, data["model_price"])
	assert.Equal(t, map[string]any{"token-model": 5.0}, data["completion_ratio"])
	assert.Equal(t, map[string]any{"token-model": 0.1}, data["cache_ratio"])
}

func TestNormalizePricingResponseJSONSupportsEncodedDocument(t *testing.T) {
	document := []byte(`{"success":true,"data":[]}`)
	encoded, err := common.Marshal(string(document))
	require.NoError(t, err)

	assert.Equal(t, document, normalizePricingResponseJSON(encoded))
	assert.Equal(t, document, normalizePricingResponseJSON(document))
}
