package config

import (
	"path/filepath"
	"testing"
)

func TestLoadPrefersUserConfigOverEnv(t *testing.T) {
	t.Parallel()

	path := writeConfigFile(t, `{
  "providers": {
    "openai": {
      "base_url": "https://config.example.com",
      "token": "config-token",
      "model": "config-model",
      "concurrency": 4
    }
  }
}`)

	loader := Loader{
		UserConfigPath: path,
		LookupEnv: func(key string) (string, bool) {
			values := map[string]string{
				"TL_OPENAI_BASE_URL":    "https://env.example.com",
				"TL_OPENAI_TOKEN":       "env-token",
				"TL_OPENAI_MODEL":       "env-model",
				"TL_OPENAI_CONCURRENCY": "16",
			}
			value, ok := values[key]
			return value, ok
		},
	}

	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Providers.OpenAI.BaseURL != "https://config.example.com" {
		t.Fatalf("BaseURL = %q, want config value", cfg.Providers.OpenAI.BaseURL)
	}
	if cfg.Providers.OpenAI.Token != "config-token" {
		t.Fatalf("Token = %q, want config value", cfg.Providers.OpenAI.Token)
	}
	if cfg.Providers.OpenAI.Model != "config-model" {
		t.Fatalf("Model = %q, want config value", cfg.Providers.OpenAI.Model)
	}
	if cfg.Providers.OpenAI.Concurrency != 4 {
		t.Fatalf("Concurrency = %d, want config value", cfg.Providers.OpenAI.Concurrency)
	}
}

func TestLoadFallsBackToEnvWhenUserConfigMissing(t *testing.T) {
	t.Parallel()

	loader := Loader{
		UserConfigPath: filepath.Join(t.TempDir(), "missing.json"),
		LookupEnv: func(key string) (string, bool) {
			values := map[string]string{
				"TL_OPENAI_BASE_URL":    "https://env.example.com",
				"TL_OPENAI_TOKEN":       "env-token",
				"TL_OPENAI_MODEL":       "env-model",
				"TL_OPENAI_CONCURRENCY": "16",
			}
			value, ok := values[key]
			return value, ok
		},
	}

	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Providers.OpenAI.BaseURL != "https://env.example.com" {
		t.Fatalf("BaseURL = %q, want env value", cfg.Providers.OpenAI.BaseURL)
	}
	if cfg.Providers.OpenAI.Token != "env-token" {
		t.Fatalf("Token = %q, want env value", cfg.Providers.OpenAI.Token)
	}
	if cfg.Providers.OpenAI.Model != "env-model" {
		t.Fatalf("Model = %q, want env value", cfg.Providers.OpenAI.Model)
	}
	if cfg.Providers.OpenAI.Concurrency != 16 {
		t.Fatalf("Concurrency = %d, want env value", cfg.Providers.OpenAI.Concurrency)
	}
}

func TestLoadDefaultsConcurrencyToEightWhenMissingOrInvalid(t *testing.T) {
	t.Parallel()

	configPath := writeConfigFile(t, `{
  "providers": {
    "openai": {
      "base_url": "https://config.example.com",
      "token": "config-token",
      "model": "config-model",
      "concurrency": 0
    }
  }
}`)

	loader := Loader{
		UserConfigPath: configPath,
		LookupEnv: func(key string) (string, bool) {
			if key == "TL_OPENAI_CONCURRENCY" {
				return "bad-value", true
			}
			return "", false
		},
	}

	cfg, err := loader.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Providers.OpenAI.Concurrency != 8 {
		t.Fatalf("Concurrency = %d, want default 8", cfg.Providers.OpenAI.Concurrency)
	}
}
