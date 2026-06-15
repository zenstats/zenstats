package service

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/zenstats/zenstats/config"
	"github.com/zenstats/zenstats/internal/store/postgresql"
	"github.com/zenstats/zenstats/internal/store/postgresql/ent/systemconfig"
	"github.com/zenstats/zenstats/pkg/globals"
)

var (
	systemConfigServiceInstance *SystemConfigService
	systemConfigOnce            sync.Once
)

type SystemConfigService struct {
	db *postgresql.Client
}

func GetSystemConfigService() *SystemConfigService {
	systemConfigOnce.Do(func() {
		db := globals.GetDB()
		if db == nil {
			panic("DB is not initialized")
		}
		systemConfigServiceInstance = &SystemConfigService{db: db}
	})
	return systemConfigServiceInstance
}

// ConfigItem 配置项
type ConfigItem struct {
	Key         string `json:"key"`
	Value       string `json:"value"`
	Description string `json:"description"`
	Group       string `json:"group_name"`
	UpdatedAt   string `json:"updated_at,omitempty"`
}

// ConfigGroup 配置分组
type ConfigGroup struct {
	Group  string       `json:"group"`
	Items  []ConfigItem `json:"items"`
}

// 预定义配置项
var configDefinitions = map[string]struct {
	Description string
	Group       string
	Default     string
}{
	"general.base_url":              {Description: "站点地址，用于生成验证链接等", Group: "general"},
	"general.admin_email":           {Description: "管理员邮箱", Group: "general"},
	"general.registration_enabled":  {Description: "是否开启注册", Group: "general", Default: "true"},
	"smtp.host":                     {Description: "SMTP 服务器地址", Group: "smtp"},
	"smtp.port":                     {Description: "SMTP 端口", Group: "smtp"},
	"smtp.username":                 {Description: "SMTP 用户名", Group: "smtp"},
	"smtp.password":                 {Description: "SMTP 密码", Group: "smtp"},
	"smtp.from":                     {Description: "发件人地址", Group: "smtp"},
}

// InitDefaults 初始化默认配置（如果数据库中不存在）
func (s *SystemConfigService) InitDefaults(ctx context.Context) {
	for key, def := range configDefinitions {
		exists, err := s.db.Client.SystemConfig.Query().
			Where(systemconfig.Key(key)).
			Exist(ctx)
		if err != nil {
			slog.Warn("failed to check system config", "key", key, "error", err)
			continue
		}
		if !exists {
			defaultValue := def.Default
			if defaultValue == "" {
				defaultValue = ""
			}
			_, err = s.db.Client.SystemConfig.Create().
				SetKey(key).
				SetValue(defaultValue).
				SetDescription(def.Description).
				SetGroupName(def.Group).
				Save(ctx)
			if err != nil {
				slog.Warn("failed to create system config", "key", key, "error", err)
			}
		}
	}
}

// LoadConfigsFromDB 从数据库加载配置到内存
func (s *SystemConfigService) LoadConfigsFromDB(ctx context.Context) error {
	configs, err := s.db.Client.SystemConfig.Query().All(ctx)
	if err != nil {
		return fmt.Errorf("failed to load system configs: %w", err)
	}

	for _, c := range configs {
		if c.Value == "" {
			continue
		}
		// 映射数据库配置到 viper
		switch c.Key {
		case "general.base_url":
			config.SetConfigValue("base_url", c.Value)
		case "general.admin_email":
			// admin_email 可扩展
		case "smtp.host":
			config.SetConfigValue("smtp.host", c.Value)
		case "smtp.port":
			// port 需要特殊处理
		case "smtp.username":
			config.SetConfigValue("smtp.username", c.Value)
		case "smtp.password":
			config.SetConfigValue("smtp.password", c.Value)
		case "smtp.from":
			config.SetConfigValue("smtp.from", c.Value)
		}
	}

	return nil
}

// GetAllConfigs 获取所有配置（按分组）
func (s *SystemConfigService) GetAllConfigs(ctx context.Context) ([]ConfigGroup, error) {
	configs, err := s.db.Client.SystemConfig.Query().All(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get configs: %w", err)
	}

	groupMap := make(map[string][]ConfigItem)
	for _, c := range configs {
		groupMap[c.GroupName] = append(groupMap[c.GroupName], ConfigItem{
			Key:         c.Key,
			Value:       c.Value,
			Description: c.Description,
			Group:       c.GroupName,
			UpdatedAt:   c.UpdatedAt.Format(time.RFC3339),
		})
	}

	// 保持预定义顺序
	var groups []ConfigGroup
	seen := make(map[string]bool)
	for _, def := range configDefinitions {
		if seen[def.Group] {
			continue
		}
		seen[def.Group] = true
		groups = append(groups, ConfigGroup{
			Group: def.Group,
			Items: groupMap[def.Group],
		})
	}

	return groups, nil
}

// UpdateConfigs 批量更新配置
func (s *SystemConfigService) UpdateConfigs(ctx context.Context, items []ConfigItem) error {
	for _, item := range items {
		// 验证 key 是否合法
		if _, ok := configDefinitions[item.Key]; !ok {
			return fmt.Errorf("invalid config key: %s", item.Key)
		}

		// 更新数据库
		_, err := s.db.Client.SystemConfig.Update().
			Where(systemconfig.Key(item.Key)).
			SetValue(item.Value).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to update config %s: %w", item.Key, err)
		}

		// 同步到内存配置
		switch item.Key {
		case "general.base_url":
			config.SetConfigValue("base_url", item.Value)
		case "smtp.host":
			config.SetConfigValue("smtp.host", item.Value)
		case "smtp.username":
			config.SetConfigValue("smtp.username", item.Value)
		case "smtp.password":
			config.SetConfigValue("smtp.password", item.Value)
		case "smtp.from":
			config.SetConfigValue("smtp.from", item.Value)
		}
	}

	return nil
}

// IsRegistrationEnabled 检查注册功能是否开启。
func (s *SystemConfigService) IsRegistrationEnabled(ctx context.Context) bool {
	cfg, err := s.db.Client.SystemConfig.Query().
		Where(systemconfig.Key("general.registration_enabled")).
		Only(ctx)
	if err != nil {
		return true
	}
	return cfg.Value != "false"
}
