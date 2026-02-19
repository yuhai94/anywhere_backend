package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	AWS       AWSConfig       `yaml:"aws"`
	V2Ray     V2RayConfig     `yaml:"v2ray"`
	Logging   LoggingConfig   `yaml:"logging"`
	Scheduler SchedulerConfig `yaml:"scheduler"`
}

type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

type DatabaseConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	DBName   string `yaml:"dbname"`
}

type AWSConfig struct {
	AccessKey string                     `yaml:"access_key"`
	SecretKey string                     `yaml:"secret_key"`
	Regions   map[string]AWSRegionConfig `yaml:"regions"`
}

type AWSRegionConfig struct {
	TemplateID string `yaml:"template_id"`
	Name       string `yaml:"name"`
}

type V2RayConfig struct {
	LocalConfigPath string `yaml:"local_config_path"`
	Port            int    `yaml:"port"`
}

type SchedulerConfig struct {
	InstanceSyncInterval int `yaml:"instance_sync_interval"`
	InstanceWaitTimeout  int `yaml:"instance_wait_timeout"`
}

type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

var AppConfig *Config

// LoadConfig 加载配置文件
// 参数:
//   - configPath: 配置文件路径，如果为空则使用默认路径 "conf/conf.yaml"
//
// 返回值:
//   - error: 错误信息，如果加载失败
//
// 功能:
//  1. 如果未指定配置路径，使用默认路径
//  2. 获取配置文件的绝对路径
//  3. 读取配置文件内容
//  4. 解析 YAML 配置
//  5. 将配置保存到全局变量 AppConfig
func LoadConfig(configPath string) error {
	if configPath == "" {
		configPath = "conf/conf.yaml"
	}

	absPath, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %v", err)
	}

	data, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %v", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %v", err)
	}

	AppConfig = &config
	return nil
}

// GetDSN 生成数据库连接字符串
// 返回值:
//   - string: 数据库连接字符串
//
// 功能:
//  1. 使用配置文件中的数据库信息生成连接字符串
//  2. 包含用户名、密码、主机、端口、数据库名等信息
//  3. 设置字符集为 utf8mb4，启用时间解析，使用本地时区
func GetDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		AppConfig.Database.User,
		AppConfig.Database.Password,
		AppConfig.Database.Host,
		AppConfig.Database.Port,
		AppConfig.Database.DBName,
	)
}

// GetRegionConfig 获取指定 AWS 区域的配置
// 参数:
//   - region: AWS 区域名称
//
// 返回值:
//   - *AWSRegionConfig: 区域配置信息
//   - error: 错误信息，如果区域未配置
//
// 功能:
//  1. 检查指定区域是否在配置中
//  2. 如果区域存在，返回其配置信息
//  3. 如果区域不存在，返回错误
func GetRegionConfig(region string) (*AWSRegionConfig, error) {
	if config, ok := AppConfig.AWS.Regions[region]; ok {
		return &config, nil
	}
	return nil, fmt.Errorf("region %s not configured", region)
}
