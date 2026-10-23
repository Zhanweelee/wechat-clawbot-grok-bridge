package session

import (
	"encoding/json"
	"os"
	"time"

	"github.com/Zhanweelee/wechat-clawbot-grok-bridge/internal/ilink"
)

// Store persists bot session on disk.
type Store struct {
	Path string
}

// Load reads session from disk.
func (s Store) Load() (*ilink.Session, error) {
	data, err := os.ReadFile(s.Path)
	if err != nil {
		return nil, err
	}
	var sess ilink.Session
	if err := json.Unmarshal(data, &sess); err != nil {
		return nil, err
	}
	return &sess, nil
}

// Save writes session with mode 0600.
func (s Store) Save(sess *ilink.Session) error {
	if sess.SavedAt == "" {
		sess.SavedAt = time.Now().UTC().Format(time.RFC3339)
	}
	data, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.Path, data, 0o600)
}

// LoadCursor reads long-poll cursor.
func LoadCursor(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// SaveCursor persists long-poll cursor.
func SaveCursor(path, cursor string) error {
	return os.WriteFile(path, []byte(cursor), 0o600)
}
