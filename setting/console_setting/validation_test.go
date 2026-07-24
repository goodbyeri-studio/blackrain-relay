package console_setting

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAnnouncementsOnlyReturnsManuallyPublishedEntries(t *testing.T) {
	settings := GetConsoleSetting()
	original := settings.Announcements
	t.Cleanup(func() {
		settings.Announcements = original
	})

	settings.Announcements = `[
		{"id":1,"content":"legacy entry","publishDate":"2026-07-22T10:00:00Z","type":"default"},
		{"id":2,"content":"draft entry","publishDate":"2026-07-24T10:00:00Z","type":"warning","published":false},
		{"id":3,"content":"published entry","publishDate":"2026-07-23T10:00:00Z","type":"success","published":true}
	]`

	announcements := GetAnnouncements()
	require.Len(t, announcements, 2)
	assert.Equal(t, float64(3), announcements[0]["id"])
	assert.Equal(t, float64(1), announcements[1]["id"])
}

func TestValidateAnnouncementsRejectsInvalidPublishedState(t *testing.T) {
	err := ValidateConsoleSettings(
		`[{"content":"entry","publishDate":"2026-07-24T10:00:00Z","published":"yes"}]`,
		"Announcements",
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "发布状态必须为布尔值")
}

func TestValidateAnnouncementsCountsUnicodeCharacters(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		extra       string
		wantErrText string
	}{
		{
			name:    "accepts content and extra at limit",
			content: strings.Repeat("测", 500),
			extra:   strings.Repeat("注", 200),
		},
		{
			name:        "rejects content over limit",
			content:     strings.Repeat("测", 501),
			wantErrText: "内容长度不能超过500字符",
		},
		{
			name:        "rejects extra over limit",
			content:     "entry",
			extra:       strings.Repeat("注", 201),
			wantErrText: "说明长度不能超过200字符",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			settings := fmt.Sprintf(
				`[{"content":%q,"publishDate":"2026-07-24T10:00:00Z","extra":%q}]`,
				test.content,
				test.extra,
			)
			err := ValidateConsoleSettings(settings, "Announcements")

			if test.wantErrText == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.wantErrText)
		})
	}
}
