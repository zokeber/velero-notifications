package config

import (
	"io/ioutil"
	"gopkg.in/yaml.v2"
)

type Config struct {
	Logging struct {
		Level string `yaml:"level"`
		Verbose bool `yaml:"verbose"`
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
			Enabled      bool   `yaml:"enabled"`
			FailuresOnly bool   `yaml:"failures_only"`
			SMTPServer 	 string `yaml:"smtp_server"`
			SMTPPort     int    `yaml:"smtp_port"`
			Username     string `yaml:"username"`
			Password     string `yaml:"password"`
			From         string `yaml:"from"`
			To           string `yaml:"to"`
		} `yaml:"email"`
	} `yaml:"notifications"`
}

func LoadConfig(path string) (*Config, error) {
	
	data, err := ioutil.ReadFile(path)
	
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