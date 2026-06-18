package project

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"matrix/internal/platform/config"
	"matrix/internal/platform/db/models"
)

// IntegrationSettings 项目级 MCP / 模型覆盖（存 PG JSONB）。
type IntegrationSettings struct {
	Model      *ModelOverride               `json:"model,omitempty"`
	MCPServers map[string]MCPServerOverride `json:"mcp_servers,omitempty"`
}

// ModelOverride 是项目级 LLM 模型覆盖配置。
type ModelOverride struct {
	BaseURL   string `json:"base_url,omitempty"`
	APIKey    string `json:"api_key,omitempty"`
	Model     string `json:"model,omitempty"`
	MaxTokens int    `json:"max_tokens,omitempty"`
}

// MCPServerOverride 是项目级单个 MCP 服务器覆盖配置。
type MCPServerOverride struct {
	Command  string            `json:"command,omitempty"`
	Args     []string          `json:"args,omitempty"`
	URL      string            `json:"url,omitempty"`
	Disabled bool              `json:"disabled,omitempty"`
	Headers  map[string]string `json:"headers,omitempty"`
}

// SettingsService 管理项目集成设置（模型与 MCP 覆盖）的读写。
type SettingsService struct {
	db *gorm.DB
}

// NewSettingsService 创建项目设置服务实例。
func NewSettingsService(db *gorm.DB) *SettingsService {
	return &SettingsService{db: db}
}

// GetIntegrations 返回项目集成设置。
func (s *SettingsService) GetIntegrations(ctx context.Context, projectID uuid.UUID) (*IntegrationSettings, error) {
	return s.Get(ctx, projectID)
}

// Get 执行对应操作。
func (s *SettingsService) Get(ctx context.Context, projectID uuid.UUID) (*IntegrationSettings, error) {
	var row models.ProjectSetting
	err := s.db.WithContext(ctx).First(&row, "project_id = ?", projectID).Error
	if err == gorm.ErrRecordNotFound {
		return &IntegrationSettings{}, nil
	}
	if err != nil {
		return nil, err
	}
	var out IntegrationSettings
	if row.Settings != "" {
		_ = json.Unmarshal([]byte(row.Settings), &out)
	}
	return &out, nil
}

// Save 保存数据。
func (s *SettingsService) Save(ctx context.Context, projectID uuid.UUID, in IntegrationSettings) (*IntegrationSettings, error) {
	raw, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	row := models.ProjectSetting{
		ProjectID: projectID,
		Settings:  string(raw),
		UpdatedAt: time.Now(),
	}
	if err := s.db.WithContext(ctx).Save(&row).Error; err != nil {
		return nil, err
	}
	return &in, nil
}

// MergeModel 将项目覆盖合并到全局默认模型。
func MergeModel(global config.ModelYAML, override *ModelOverride) config.ModelYAML {
	if override == nil {
		return global
	}
	out := global
	if override.BaseURL != "" {
		out.BaseURL = override.BaseURL
	}
	if override.APIKey != "" {
		out.APIKey = override.APIKey
	}
	if override.Model != "" {
		out.Model = override.Model
	}
	if override.MaxTokens > 0 {
		out.MaxTokens = override.MaxTokens
	}
	return out
}

// MergeMCP 将项目 MCP 覆盖合并到全局配置。
func MergeMCP(global map[string]config.MCPServerYAML, override map[string]MCPServerOverride) map[string]config.MCPServerYAML {
	out := make(map[string]config.MCPServerYAML, len(global)+len(override))
	for k, v := range global {
		out[k] = v
	}
	for name, o := range override {
		base := out[name]
		if o.Command != "" {
			base.Command = o.Command
		}
		if len(o.Args) > 0 {
			base.Args = o.Args
		}
		if o.URL != "" {
			base.URL = o.URL
		}
		if o.Headers != nil {
			base.Headers = o.Headers
		}
		base.Disabled = o.Disabled
		out[name] = base
	}
	return out
}
