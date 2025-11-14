package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
)

const (
	serviceName = "splunk-cli"
	configFile  = "config.json"
)

// config represents the splunk-cli configuration
type config struct {
	Host string `json:"host"`
}

// getConfigPath returns the path to the config file
func getConfigPath() (string, error) {
	configDirPath, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to get config directory: %w", err)
	}

	configPath := filepath.Join(configDirPath, "splunk-cli", configFile)
	return configPath, nil
}

// SaveConfig saves the host to the config file
func SaveConfig(host string) error {
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	// Create config directory if it doesn't exist
	configDirPath := filepath.Dir(configPath)
	if err := os.MkdirAll(configDirPath, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	cfg := config{Host: host}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// LoadConfig loads the host from the config file
func LoadConfig() (string, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return "", err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("failed to parse config file: %w", err)
	}

	return cfg.Host, nil
}

// SaveToken saves the token to the keyring
func SaveToken(host, token string) error {
	return keyring.Set(serviceName, host, token)
}

// LoadToken loads the token from the keyring
func LoadToken(host string) (string, error) {
	return keyring.Get(serviceName, host)
}
