package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
)

type Config struct {
	Providers Providers `json:"providers"`
}

type Providers struct {
	OpenAI OpenAIConfig `json:"openai"`
}

type OpenAIConfig struct {
	BaseURL      string `json:"base_url"`
	Token        string `json:"token"`
	Model        string `json:"model"`
	CustomPrompt string `json:"custom_prompt"`
	Concurrency  int    `json:"concurrency"`
}

type Loader struct {
	UserConfigPath string
	LookupEnv      func(string) (string, bool)
}

func (l Loader) Load() (Config, error) {
	cfg := Config{}
	path := l.UserConfigPath
	if path == "" {
		defaultPath, err := defaultUserConfigPath()
		if err != nil {
			return Config{}, err
		}
		path = defaultPath
	}

	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			return Config{}, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Config{}, err
	}

	applyEnvFallback(&cfg, valueOrDefaultEnv(l.LookupEnv, os.LookupEnv))
	applyDefaults(&cfg)
	return cfg, nil
}

func defaultUserConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".config", "tl", "config.json"), nil
}

func applyEnvFallback(cfg *Config, lookupEnv func(string) (string, bool)) {
	setIfEmpty(&cfg.Providers.OpenAI.BaseURL, lookupEnv, "TL_OPENAI_BASE_URL", "OPENAI_BASE_URL")
	setIfEmpty(&cfg.Providers.OpenAI.Token, lookupEnv, "TL_OPENAI_TOKEN", "OPENAI_API_KEY")
	setIfEmpty(&cfg.Providers.OpenAI.Model, lookupEnv, "TL_OPENAI_MODEL", "OPENAI_MODEL")
	setIfEmpty(&cfg.Providers.OpenAI.CustomPrompt, lookupEnv, "TL_OPENAI_CUSTOM_PROMPT")
	setIntIfEmpty(&cfg.Providers.OpenAI.Concurrency, lookupEnv, "TL_OPENAI_CONCURRENCY", "OPENAI_CONCURRENCY")
}

func setIfEmpty(target *string, lookupEnv func(string) (string, bool), keys ...string) {
	if *target != "" {
		return
	}
	for _, key := range keys {
		if value, ok := lookupEnv(key); ok && value != "" {
			*target = value
			return
		}
	}
}

func setIntIfEmpty(target *int, lookupEnv func(string) (string, bool), keys ...string) {
	if *target > 0 {
		return
	}
	for _, key := range keys {
		value, ok := lookupEnv(key)
		if !ok || value == "" {
			continue
		}
		parsed, err := strconv.Atoi(value)
		if err == nil && parsed > 0 {
			*target = parsed
			return
		}
	}
}

func applyDefaults(cfg *Config) {
	if cfg.Providers.OpenAI.Concurrency <= 0 {
		cfg.Providers.OpenAI.Concurrency = 8
	}
}

func valueOrDefaultEnv(current func(string) (string, bool), alt func(string) (string, bool)) func(string) (string, bool) {
	if current != nil {
		return current
	}
	return alt
}
