package localv2ray

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/yuhai94/anywhere_backend/internal/logging"
)

type V2RayConfig struct {
	Log       LogConfig        `json:"log"`
	Stats     StatsConfig      `json:"stats"`
	API       APIConfig        `json:"api"`
	Policy    PolicyConfig     `json:"policy"`
	Inbounds  []InboundConfig  `json:"inbounds,omitempty"`
	Outbounds []OutboundConfig `json:"outbounds,omitempty"`
	Routing   RoutingConfig    `json:"routing,omitempty"`
}

type LogConfig struct {
	Access   string `json:"access,omitempty"`
	Error    string `json:"error,omitempty"`
	LogLevel string `json:"loglevel,omitempty"`
}

type StatsConfig struct {
}

type APIConfig struct {
	Tag      string   `json:"tag,omitempty"`
	Services []string `json:"services,omitempty"`
}

type PolicyConfig struct {
	Levels map[string]LevelConfig `json:"levels,omitempty"`
	System SystemPolicyConfig     `json:"system,omitempty"`
}

type LevelConfig struct {
	StatsUserUplink   bool `json:"statsUserUplink,omitempty"`
	StatsUserDownlink bool `json:"statsUserDownlink,omitempty"`
}

type SystemPolicyConfig struct {
	StatsInboundUplink    bool `json:"statsInboundUplink,omitempty"`
	StatsInboundDownlink  bool `json:"statsInboundDownlink,omitempty"`
	StatsOutboundUplink   bool `json:"statsOutboundUplink,omitempty"`
	StatsOutboundDownlink bool `json:"statsOutboundDownlink,omitempty"`
}

type InboundConfig struct {
	Tag      string                `json:"tag,omitempty"`
	Port     int                   `json:"port,omitempty"`
	Protocol string                `json:"protocol,omitempty"`
	Listen   string                `json:"listen,omitempty"`
	Settings *VmessInboundSettings `json:"settings,omitempty"`
}

type VmessInboundSettings struct {
	Clients []VmessClientConfig `json:"clients,omitempty"`
}

type VmessClientConfig struct {
	AlterId int    `json:"alterId,omitempty"`
	Email   string `json:"email,omitempty"`
	ID      string `json:"id,omitempty"`
}

type OutboundConfig struct {
	Protocol string      `json:"protocol,omitempty"`
	Tag      string      `json:"tag,omitempty"`
	Settings interface{} `json:"settings,omitempty"`
}

type RoutingConfig struct {
	DomainStrategy string           `json:"domainStrategy,omitempty"`
	Balancers      []BalancerConfig `json:"balancers,omitempty"`
	Strategy       string           `json:"strategy,omitempty"`
	Rules          []RuleConfig     `json:"rules,omitempty"`
}

type BalancerConfig struct {
	Tag      string   `json:"tag,omitempty"`
	Selector []string `json:"selector,omitempty"`
}

type RuleConfig struct {
	InboundTag  []string `json:"inboundTag,omitempty"`
	OutboundTag string   `json:"outboundTag,omitempty"`
	Type        string   `json:"type,omitempty"`
	IP          []string `json:"ip,omitempty"`
	Domain      []string `json:"domain,omitempty"`
	User        []string `json:"user,omitempty"`
	BalancerTag string   `json:"balancerTag,omitempty"`
	Network     string   `json:"network,omitempty"`
}

type VNextConfig struct {
	Address string       `json:"address,omitempty"`
	Port    int          `json:"port,omitempty"`
	Users   []UserConfig `json:"users,omitempty"`
}

type UserConfig struct {
	ID      string `json:"id,omitempty"`
	AlterId int    `json:"alterId,omitempty"`
}

type VmessOutboundSettings struct {
	VNext []VNextConfig `json:"vnext,omitempty"`
}

type LocalV2RayManager struct {
	configPath string
}

// NewLocalV2RayManager 创建一个新的 LocalV2RayManager 实例
// 参数:
//   - configPath: 本地 V2Ray 配置文件路径
//
// 返回值:
//   - *LocalV2RayManager: 新创建的 LocalV2RayManager 实例
//
// 功能:
//  1. 初始化 LocalV2RayManager 结构体
//  2. 设置配置文件路径
func NewLocalV2RayManager(configPath string) *LocalV2RayManager {
	return &LocalV2RayManager{
		configPath: configPath,
	}
}

// AddInstance 向本地 V2Ray 配置添加一个实例
// 参数:
//   - ctx: 上下文，用于传递请求范围的值和取消信号
//   - instanceTag: 实例标签
//   - address: 实例地址
//   - port: 实例端口
//   - uuid: 实例 UUID
//
// 返回值:
//   - error: 错误信息，如果添加失败
//
// 功能:
//  1. 读取当前 V2Ray 配置
//  2. 创建新的出站配置
//  3. 检查是否已存在相同标签的出站配置
//  4. 如果存在，更新配置；如果不存在，添加新配置
//  5. 写回配置文件
//  6. 重启 V2Ray 服务
func (m *LocalV2RayManager) AddInstance(ctx context.Context, instanceTag, address string, port int, uuid string) error {
	// Read current config
	config, err := m.ReadConfig()
	if err != nil {
		logging.Error(ctx, "Failed to read local V2Ray config: %v", err)
		return fmt.Errorf("failed to read local V2Ray config: %v", err)
	}

	// Create new outbound
	newOutbound := OutboundConfig{
		Protocol: "vmess",
		Tag:      instanceTag,
		Settings: VmessOutboundSettings{
			VNext: []VNextConfig{
				{
					Address: address,
					Port:    port,
					Users: []UserConfig{
						{
							ID:      uuid,
							AlterId: 0,
						},
					},
				},
			},
		},
	}

	// Check if outbound already exists
	found := false
	for i, outbound := range config.Outbounds {
		if outbound.Tag == instanceTag {
			config.Outbounds[i] = newOutbound
			found = true
			break
		}
	}

	// Add new outbound if not exists
	if !found {
		config.Outbounds = append(config.Outbounds, newOutbound)
	}

	// Write config back
	if err := m.WriteConfig(config); err != nil {
		logging.Error(ctx, "Failed to write local V2Ray config: %v", err)
		return fmt.Errorf("failed to write local V2Ray config: %v", err)
	}

	// Restart V2Ray service
	if err := m.RestartService(ctx); err != nil {
		logging.Error(ctx, "Failed to restart V2Ray service: %v", err)
		// Continue even if service restart fails
	}

	logging.Info(ctx, "Added V2Ray instance %s to local config", instanceTag)
	return nil
}

// ReadConfig 读取本地 V2Ray 配置文件
// 返回值:
//   - *V2RayConfig: 解析后的 V2Ray 配置
//   - error: 错误信息，如果读取或解析失败
//
// 功能:
//  1. 读取配置文件内容
//  2. 解析 JSON 格式的配置
//  3. 返回解析后的配置对象
func (m *LocalV2RayManager) ReadConfig() (*V2RayConfig, error) {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	var config V2RayConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %v", err)
	}

	return &config, nil
}

// WriteConfig 写入本地 V2Ray 配置文件
// 参数:
//   - config: 要写入的 V2Ray 配置
//
// 返回值:
//   - error: 错误信息，如果写入失败
//
// 功能:
//  1. 将配置对象序列化为 JSON 格式
//  2. 创建配置文件的备份
//  3. 写入新的配置文件
//  4. 如果写入失败，恢复备份
//  5. 如果写入成功，删除备份
func (m *LocalV2RayManager) WriteConfig(config *V2RayConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %v", err)
	}

	// Create backup
	backupPath := m.configPath + ".bak"
	if err := os.Rename(m.configPath, backupPath); err != nil {
		return fmt.Errorf("failed to create backup: %v", err)
	}

	// Write new config
	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		// Restore backup
		os.Rename(backupPath, m.configPath)
		return fmt.Errorf("failed to write config: %v", err)
	}

	// Remove backup
	os.Remove(backupPath)
	return nil
}

// RestartService 重启本地 V2Ray 服务
// 参数:
//   - ctx: 上下文，用于传递请求范围的值和取消信号
//
// 返回值:
//   - error: 错误信息，如果重启失败
//
// 功能:
//  1. 执行系统命令重启 V2Ray 服务
//  2. 记录重启过程和结果
//  3. 返回重启操作的错误信息
func (m *LocalV2RayManager) RestartService(ctx context.Context) error {
	logging.Info(ctx, "Restarting V2Ray service...")

	cmd := exec.Command("sudo", "systemctl", "restart", "v2ray")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logging.Error(ctx, "Failed to restart V2Ray service: %v, output: %s", err, string(output))
		return fmt.Errorf("failed to restart V2Ray service: %v", err)
	}

	logging.Info(ctx, "V2Ray service restarted successfully")
	return nil
}

// ReloadConfig 重新加载本地 V2Ray 配置
// 参数:
//   - ctx: 上下文，用于传递请求范围的值和取消信号
//
// 返回值:
//   - error: 错误信息，如果重新加载失败
//
// 功能:
//  1. 记录配置已更新的信息
//  2. 提示需要重新加载 V2Ray 服务
//  3. 注意：在生产环境中，应该使用 V2Ray API 来重新加载配置
func (m *LocalV2RayManager) ReloadConfig(ctx context.Context) error {
	// In production, you would use V2Ray API to reload config
	// For now, we'll just log that a reload is needed
	logging.Info(ctx, "Local V2Ray config updated. Please reload V2Ray service.")
	return nil
}

// GetRelayConfig 获取本地 V2Ray 的中转配置
// 参数:
//   - region: AWS 区域名称
//
// 返回值:
//   - int: 中转端口
//   - string: 中转 UUID
//   - error: 错误信息
//
// 功能:
//  1. 读取本地 V2Ray 配置文件
//  2. 查找 protocol="vmess" 的 inbound
//  3. 匹配 email 为 "user_aws_"+region 的 client
//  4. 返回找到的端口和 UUID
func (m *LocalV2RayManager) GetRelayConfig(region string) (int, string, error) {
	config, err := m.ReadConfig()
	if err != nil {
		return 0, "", fmt.Errorf("failed to read config: %v", err)
	}

	expectedEmail := "user_aws_" + region

	for _, inbound := range config.Inbounds {
		if inbound.Protocol == "vmess" && inbound.Settings != nil {
			for _, client := range inbound.Settings.Clients {
				if client.Email == expectedEmail {
					return inbound.Port, client.ID, nil
				}
			}
		}
	}

	return 0, "", fmt.Errorf("relay config not found for region %s", region)
}
