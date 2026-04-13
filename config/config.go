package config

import (
	"fmt"
	"os"
	"strings"
	"gopkg.in/yaml.v2"
)

type EmailRecipients []string

func (r *EmailRecipients) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var raw interface{}
	if err := unmarshal(&raw); err != nil {
		return err
	}

	switch v := raw.(type) {
	case string:
		if v == "" {
			*r = nil
			return nil
		}
		parts := strings.Split(v, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		*r = parts
		return nil
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				result = append(result, strings.TrimSpace(str))
			} else {
				return fmt.Errorf("email recipient must be a string")
			}
		}
		*r = result
		return nil
	default:
		return fmt.Errorf("unsupported yaml type for email recipients: %T", raw)
	}
}

type Config struct {
	Logging struct {
		Level   string `yaml:"level"`
		Verbose bool   `yaml:"verbose"`
	} `yaml:"logging"`
	Namespace     string `yaml:"namespace"`
	CheckInterval int    `yaml:"check_interval"`
	Notifications struct {
		NotificationPrefix string `yaml:"notification_prefix"`
		Slack              struct {
			Enabled      bool   `yaml:"enabled"`
			FailuresOnly bool   `yaml:"failures_only"`
			Webhook      string `yaml:"webhook_url"`
			Channel      string `yaml:"channel"`
			Username     string `yaml:"username"`
		} `yaml:"slack"`
		Email struct {
			Enabled      bool            `yaml:"enabled"`
			FailuresOnly bool            `yaml:"failures_only"`
			SMTPServer   string          `yaml:"smtp_server"`
			SMTPPort     int             `yaml:"smtp_port"`
			Username     string          `yaml:"username"`
			Password     string          `yaml:"password"`
			From         string          `yaml:"from"`
			To           EmailRecipients `yaml:"to"`
		} `yaml:"email"`
	} `yaml:"notifications"`
}

func LoadConfig(path string) (*Config, error) {

	data, err := os.ReadFile(path)

	if err != nil {
		return nil, err
	}

	var cfg Config

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	if cfg.CheckInterval < 2 {
		cfg.CheckInterval = 2
	}

	return &cfg, nil
}
