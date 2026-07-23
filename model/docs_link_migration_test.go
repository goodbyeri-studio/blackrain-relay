package model

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestMigrateLegacyDocsLink(t *testing.T) {
	testCases := []struct {
		name     string
		current  string
		expected string
	}{
		{
			name:     "legacy URL without trailing slash",
			current:  "https://doc.deepkey.top",
			expected: operation_setting.DefaultDocsLink,
		},
		{
			name:     "legacy URL with trailing slash",
			current:  "https://doc.deepkey.top/",
			expected: operation_setting.DefaultDocsLink,
		},
		{
			name:     "custom documentation URL",
			current:  "https://docs.example.com",
			expected: "https://docs.example.com",
		},
	}

	for index, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			dsn := fmt.Sprintf("file:docs-link-migration-%d?mode=memory&cache=shared", index)
			db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
			require.NoError(t, err)
			require.NoError(t, db.AutoMigrate(&Option{}))
			require.NoError(t, db.Create(&Option{Key: docsLinkOptionKey, Value: testCase.current}).Error)
			require.NoError(t, migrateLegacyDocsLink(db))

			var option Option
			require.NoError(t, db.First(&option, "key = ?", docsLinkOptionKey).Error)
			assert.Equal(t, testCase.expected, option.Value)
		})
	}
}
