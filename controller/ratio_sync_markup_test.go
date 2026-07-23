package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
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

func TestFetchUpstreamRatiosRejectsMarkupOutsideAllowedRange(t *testing.T) {
	testCases := []struct {
		name    string
		request string
	}{
		{name: "negative", request: `{"upstreams":[{"name":"DeepKey","base_url":"https://deepkey.top"}],"markup_percent":-0.1}`},
		{name: "above maximum", request: `{"upstreams":[{"name":"DeepKey","base_url":"https://deepkey.top"}],"markup_percent":1000.1}`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodPost, "/api/ratio_sync/fetch", bytes.NewBufferString(testCase.request))
			ctx.Request.Header.Set("Content-Type", "application/json")

			FetchUpstreamRatios(ctx)

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Contains(t, recorder.Body.String(), "加价百分比必须在 0 到 1000 之间")
		})
	}
}

func TestNormalizePricingResponseJSONSupportsEncodedDocument(t *testing.T) {
	document := []byte(`{"success":true,"data":[]}`)
	encoded, err := common.Marshal(string(document))
	require.NoError(t, err)

	assert.Equal(t, document, normalizePricingResponseJSON(encoded))
	assert.Equal(t, document, normalizePricingResponseJSON(document))
}

func TestEffectivePricingMarkupPercentSkipsDeepKeyModelMarkup(t *testing.T) {
	assert.Equal(t, 0.0, effectivePricingMarkupPercent("https://deepkey.top/api/pricing", 30))
	assert.Equal(t, 0.0, effectivePricingMarkupPercent("https://DEEPKEY.TOP/api/pricing/", 30))
	assert.Equal(t, 30.0, effectivePricingMarkupPercent("https://example.com/api/pricing", 30))
	assert.Equal(t, 30.0, effectivePricingMarkupPercent("https://deepkey.top/v1/models", 30))
}
