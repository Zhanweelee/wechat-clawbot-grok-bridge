package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnvOverlay(t *testing.T) {
	t.Setenv("WCG_WEBHOOK_URL", "https://example.com/hook")
	t.Setenv("WCG_WEBHOOK_KEY", "secret-key")
	cfg := Default()
	applyEnv(&cfg)
	if cfg.WebhookURL != "https://example.com/hook" {
		t.Fatal(cfg.WebhookURL)
	}
	if cfg.WebhookAuthHeader() != "Bearer secret-key" {
		t.Fatal(cfg.WebhookAuthorization)
	}
}

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	_ = os.WriteFile(path, []byte(`{"listen_addr":"127.0.0.1:1"}`), 0o600)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ListenAddr != "127.0.0.1:1" {
		t.Fatal(cfg.ListenAddr)
	}
}
