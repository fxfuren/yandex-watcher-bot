package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the application configuration
type Config struct {
	BotToken           string        `yaml:"-"`
	GroupChatID        int64         `yaml:"-"`
	TopicID            *int          `yaml:"-"`
	MinCheckInterval   time.Duration `yaml:"-"`
	MaxCheckInterval   time.Duration `yaml:"-"`
	APIWorkerPoolSize  int           `yaml:"-"`
	TelegramWorkers    int           `yaml:"-"`
	VMs                []VM          `yaml:"vms"`
}

// VM represents a virtual machine configuration
type VM struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
	IP   string `yaml:"ip,omitempty"`
}

// Load reads configuration from environment and YAML file
func Load(yamlPath string) (*Config, error) {
	cfg := &Config{
		MinCheckInterval:  getEnvDuration("MIN_CHECK_INTERVAL", 5*time.Second),
		MaxCheckInterval:  getEnvDuration("MAX_CHECK_INTERVAL", 60*time.Second),
		APIWorkerPoolSize: getEnvInt("API_WORKER_POOL_SIZE", 10),
		TelegramWorkers:   getEnvInt("TELEGRAM_WORKERS", 3),
	}

	// Bot token (required)
	cfg.BotToken = os.Getenv("BOT_TOKEN")
	if cfg.BotToken == "" {
		return nil, fmt.Errorf("BOT_TOKEN environment variable is required")
	}

	// Group chat ID (required)
	groupChatIDStr := os.Getenv("GROUP_CHAT_ID")
	if groupChatIDStr == "" {
		return nil, fmt.Errorf("GROUP_CHAT_ID environment variable is required")
	}
	groupChatID, err := strconv.ParseInt(groupChatIDStr, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid GROUP_CHAT_ID: %w", err)
	}
	cfg.GroupChatID = groupChatID

	// Topic ID (optional)
	if topicIDStr := os.Getenv("TOPIC_ID"); topicIDStr != "" {
		topicID, err := strconv.Atoi(topicIDStr)
		if err != nil {
			return nil, fmt.Errorf("invalid TOPIC_ID: %w", err)
		}
		cfg.TopicID = &topicID
	}

	// Load VMs from YAML
	if err := cfg.loadVMs(yamlPath); err != nil {
		return nil, fmt.Errorf("failed to load VMs: %w", err)
	}

	return cfg, nil
}

func (c *Config) loadVMs(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Empty VMs list is acceptable
			return nil
		}
		return err
	}

	var yamlConfig struct {
		VMs []VM `yaml:"vms"`
	}

	if err := yaml.Unmarshal(data, &yamlConfig); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	c.VMs = yamlConfig.VMs
	return nil
}

// SaveVMs writes the current VM configuration back to the YAML file
func (c *Config) SaveVMs(path string) error {
	data := struct {
		VMs []VM `yaml:"vms"`
	}{
		VMs: c.VMs,
	}

	out, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal YAML: %w", err)
	}

	if err := os.WriteFile(path, out, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if val := os.Getenv(key); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
		// Try parsing as seconds
		if seconds, err := strconv.Atoi(val); err == nil {
			return time.Duration(seconds) * time.Second
		}
	}
	return defaultVal
}
