package repo

import (
	"context"
	"encoding/json"
	"time"

	"matrix/internal/platform/db/models"

	"gorm.io/gorm"
)

// SystemSettingRepo 封装 SystemSetting 表持久化操作。
type SystemSettingRepo struct {
	db *gorm.DB
}

// NewSystemSettingRepo 创建 SystemSettingRepo。
func NewSystemSettingRepo(db *gorm.DB) *SystemSettingRepo {
	return &SystemSettingRepo{db: db}
}

// SaveDomain 将指定域配置序列化写入 system_settings 表。
func (r *SystemSettingRepo) SaveDomain(ctx context.Context, domainID string, data any) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	row := models.SystemSetting{
		ID:        domainID,
		Settings:  string(raw),
		UpdatedAt: time.Now(),
	}
	return r.db.WithContext(ctx).Save(&row).Error
}

// LoadDomainRaw 从数据库读取指定域的原始 JSON；无记录时返回 (nil, nil)。
func (r *SystemSettingRepo) LoadDomainRaw(ctx context.Context, domainID string) (json.RawMessage, error) {
	var row models.SystemSetting
	err := r.db.WithContext(ctx).Where("id = ?", domainID).First(&row).Error
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if row.Settings == "" || row.Settings == "{}" {
		return nil, nil
	}
	return json.RawMessage(row.Settings), nil
}
