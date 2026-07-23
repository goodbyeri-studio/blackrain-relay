package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitOptionMapNotice(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Option{}))

	testCases := []struct {
		name     string
		stored   *Option
		expected string
	}{
		{
			name:     "uses default when option is absent",
			expected: defaultNotice,
		},
		{
			name:     "uses default when stored option is empty",
			stored:   &Option{Key: "Notice", Value: ""},
			expected: defaultNotice,
		},
		{
			name:     "preserves administrator notice",
			stored:   &Option{Key: "Notice", Value: "管理员自定义通知"},
			expected: "管理员自定义通知",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			require.NoError(t, DB.Where("1 = 1").Delete(&Option{}).Error)
			if testCase.stored != nil {
				require.NoError(t, DB.Create(testCase.stored).Error)
			}

			InitOptionMap()

			common.OptionMapRWMutex.RLock()
			actual := common.OptionMap["Notice"]
			common.OptionMapRWMutex.RUnlock()
			assert.Equal(t, testCase.expected, actual)
		})
	}
}
