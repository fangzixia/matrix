package repo

import (
	"errors"

	"gorm.io/gorm"
)

// IsNotFound 判断是否为记录不存在错误（业务层勿直接依赖 gorm.ErrRecordNotFound）。
func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
