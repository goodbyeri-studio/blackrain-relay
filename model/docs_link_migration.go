package model

import (
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

const docsLinkOptionKey = "general_setting.docs_link"

var legacyDeepKeyDocsLinks = []string{
	"https://doc.deepkey.top",
	"https://doc.deepkey.top/",
}

func migrateLegacyDocsLink(db *gorm.DB) error {
	return db.Model(&Option{}).
		Where(&Option{Key: docsLinkOptionKey}).
		Where("value IN ?", legacyDeepKeyDocsLinks).
		Update("value", operation_setting.DefaultDocsLink).Error
}
