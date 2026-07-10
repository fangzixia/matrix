package repo

import (
	"context"
	"encoding/json"
)

// SettingsStore 封装系统配置持久化。
type SettingsStore struct {
	c *catalog
}

func newSettingsStore(c *catalog) *SettingsStore { return &SettingsStore{c: c} }

func (s *SettingsStore) SaveDomain(ctx context.Context, domainID string, data any) error {
	return s.c.systemSetting.SaveDomain(ctx, domainID, data)
}

func (s *SettingsStore) LoadDomainRaw(ctx context.Context, domainID string) (json.RawMessage, error) {
	return s.c.systemSetting.LoadDomainRaw(ctx, domainID)
}
