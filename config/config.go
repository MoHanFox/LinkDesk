package config

// 我直接复制task9了

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

var Config cfg

// Config 对应配置文件 config.yaml 的结构。
type cfg struct {
	Server struct {
		Port string
	}
	Database struct {
		Host     string
		Port     string
		Path     string
		User     string
		Password string
	}
	// CorsOrigins 允许跨域访问后端的前端地址(配置项 cors_origins)
	CorsOrigins []string `mapstructure:"cors_origins"`
	// TokenTTL 登录 token 的有效期(配置项 token_ttl,例如 "24h")
	TokenTTL time.Duration `mapstructure:"token_ttl"`
	// DailyLinkQuota 每个用户每天最多创建多少条短链接(配置项 daily_link_quota)
	DailyLinkQuota int `mapstructure:"daily_link_quota"`
	// PublicBaseURL 生成短链接时使用的公开地址(配置项 public_base_url)
	PublicBaseURL string `mapstructure:"public_base_url"`
	// Timezone 计算每日配额时使用的时区(配置项 timezone)
	Timezone string `mapstructure:"timezone"`
}

// LoadConfig 用 viper 读取 config.yaml;找不到文件时退回到默认值。
func LoadConfig() error {
	viper.SetConfigName("config") // 配置文件名(不带扩展名)
	viper.SetConfigType("yaml")   // 格式为 yaml
	viper.AddConfigPath(".")      // 在当前目录查找
	viper.AutomaticEnv()          // 允许用环境变量覆盖配置项

	// 默认值:即使没有 config.yaml,程序也能用这些值正常启动
	viper.SetDefault("server.port", ":8080")
	viper.SetDefault("database.host", "127.0.0.1")
	viper.SetDefault("database.port", "3306")
	viper.SetDefault("database.path", "linkDesk")
	viper.SetDefault("database.user", "root")
	viper.SetDefault("database.password", "123456789")
	// 默认不允许任何跨域来源,必须显式配置才放行
	viper.SetDefault("cors_origins", []string{})
	viper.SetDefault("token_ttl", "24h")
	viper.SetDefault("daily_link_quota", 5)
	viper.SetDefault("public_base_url", "http://localhost:8080")
	viper.SetDefault("timezone", "Asia/Shanghai")

	if err := viper.ReadInConfig(); err != nil {
		// "文件不存在"可以容忍(用默认值);其它读取错误要上报
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("[Config] 读取配置文件出错:%w", err)
		}
		fmt.Println("[Config] 提示:未找到 config.yaml,使用默认配置。")
	}

	var cfg_ cfg
	if err := viper.Unmarshal(&cfg_); err != nil {
		return fmt.Errorf("[Config] 解析配置出错:%w", err)
	}
	Config = cfg_

	return nil
}
