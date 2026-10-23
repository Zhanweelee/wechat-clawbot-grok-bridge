package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config is the bridge runtime configuration (non-secret defaults).
type Config struct {
	WebhookURL            string `json:"webhook_url"`
	WebhookAuthorization  string `json:"webhook_authorization"` // e.g. "Bearer <key>" or raw key
	ListenAddr            string `json:"listen_addr"`
	StateDir              string `json:"state_dir"`
	RouteTag              string `json:"route_tag,omitempty"`
	LongPollTimeoutMS     int    `json:"longpoll_timeout_ms"`
	EnableTypingIndicator bool   `json:"enable_typing_indicator"`
	ReplyHTTPPath         string `json:"reply_http_path"`
}

// Default returns sensible defaults.
func Default() Config {
	home, _ := os.UserHomeDir()
	state := filepath.Join(home, ".wechat-clawbot-grok-bridge")
	return Config{
		ListenAddr:            "127.0.0.1:8787",
		StateDir:              state,
		LongPollTimeoutMS:     35000,
		EnableTypingIndicator: true,
		ReplyHTTPPath:         "/v1/reply",
	}
}

// Load reads JSON config from path, overlaying env for secrets.
func Load(path string) (Config, error) {
	cfg := Default()
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return cfg, err
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			return cfg, err
		}
	}
	applyEnv(&cfg)
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("WCG_WEBHOOK_URL"); v != "" {
		cfg.WebhookURL = v
	}
	if v := os.Getenv("WCG_WEBHOOK_KEY"); v != "" {
		cfg.WebhookAuthorization = normalizeBearer(v)
	}
	if v := os.Getenv("WCG_WEBHOOK_AUTHORIZATION"); v != "" {
		cfg.WebhookAuthorization = v
	}
	if v := os.Getenv("WCG_LISTEN_ADDR"); v != "" {
		cfg.ListenAddr = v
	}
	if v := os.Getenv("WCG_STATE_DIR"); v != "" {
		cfg.StateDir = v
	}
	if v := os.Getenv("WCG_ROUTE_TAG"); v != "" {
		cfg.RouteTag = v
	}
}

func normalizeBearer(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	lower := strings.ToLower(key)
	if strings.HasPrefix(lower, "bearer ") {
		return key
	}
	return "Bearer " + key
}

// Validate checks required fields for serve mode.
func (c Config) Validate() error {
	if c.StateDir == "" {
		return fmt.Errorf("state_dir is required")
	}
	return nil
}

// SessionPath returns credential file path.
func (c Config) SessionPath() string {
	return filepath.Join(c.StateDir, "session.json")
}

// CursorPath returns getupdates cursor path.
func (c Config) CursorPath() string {
	return filepath.Join(c.StateDir, "updates.buf")
}

// EnsureStateDir creates state directory with restrictive permissions.
func (c Config) EnsureStateDir() error {
	return os.MkdirAll(c.StateDir, 0o700)
}

// WebhookAuthHeader returns the Authorization header value for outbound webhook calls.
func (c Config) WebhookAuthHeader() string {
	return strings.TrimSpace(c.WebhookAuthorization)
}
