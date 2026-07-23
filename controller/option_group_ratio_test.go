package controller

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeGroupRatioOptions(t *testing.T) {
	values, err := normalizeGroupRatioOptions(GroupRatioOptionsRequest{
		GroupRatio:              `{"gpt":1.3}`,
		TopupGroupRatio:         `{"gpt":1.1}`,
		UserUsableGroups:        `{"gpt":"GPT"}`,
		GroupGroupRatio:         `{"vip":{"gpt":0.9}}`,
		AutoGroups:              `["gpt"]`,
		DefaultUseAutoGroup:     true,
		GroupSpecialUsableGroup: `{"vip":{"+:gpt":"GPT"}}`,
	})
	require.NoError(t, err)
	require.Equal(t, `{"gpt":1.3}`, values["GroupRatio"])
	require.Equal(t, "true", values["DefaultUseAutoGroup"])
	require.Equal(t, `{"vip":{"gpt":0.9}}`, values["GroupGroupRatio"])
}

func TestNormalizeGroupRatioOptionsRejectsInvalidRatios(t *testing.T) {
	_, err := normalizeGroupRatioOptions(GroupRatioOptionsRequest{
		GroupRatio:      `{"gpt":1}`,
		GroupGroupRatio: `{"vip":{"gpt":-0.1}}`,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "GroupGroupRatio")
}

func TestNormalizeGroupRatioOptionsRejectsNullCollections(t *testing.T) {
	_, err := normalizeGroupRatioOptions(GroupRatioOptionsRequest{
		GroupRatio:       `null`,
		TopupGroupRatio:  `{}`,
		UserUsableGroups: `{}`,
		GroupGroupRatio:  `{}`,
		AutoGroups:       `[]`,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "GroupRatio")
}
