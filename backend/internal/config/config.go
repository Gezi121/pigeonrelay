package config

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DBPath               string `yaml:"db_path"`
	JWTSecret            string `yaml:"jwt_secret"`
	Port                 string `yaml:"port"`
	AdminUser            string `yaml:"admin_user"`
	AdminPass            string `yaml:"admin_pass"`
	BackupRetentionCount int    `yaml:"backup_retention_count"`
	NotificationWebhook  string `yaml:"notification_webhook"`
	CFAPIToken           string `yaml:"cf_api_token"`
	CFZoneID             string `yaml:"cf_zone_id"`
}

func Load(path string) (*Config, error) {
	c := &Config{
		DBPath:               "/app/data/nodeforge.db",
		Port:                 "3214",
		BackupRetentionCount: 2,
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			applyEnvOverrides(c)
			return c, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, err
	}
	applyEnvOverrides(c)
	return c, nil
}

func applyEnvOverrides(c *Config) {
	if v := os.Getenv("DB_PATH"); v != "" {
		c.DBPath = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		c.JWTSecret = v
	}
	if v := os.Getenv("PORT"); v != "" {
		c.Port = v
	}
	if v := os.Getenv("ADMIN_USER"); v != "" {
		c.AdminUser = v
	}
	if v := os.Getenv("ADMIN_PASS"); v != "" {
		c.AdminPass = v
	}
	if v := os.Getenv("NOTIFICATION_WEBHOOK"); v != "" {
		c.NotificationWebhook = v
	}
	if v := os.Getenv("CF_API_TOKEN"); v != "" {
		c.CFAPIToken = v
	}
	if v := os.Getenv("CF_ZONE_ID"); v != "" {
		c.CFZoneID = v
	}
}
