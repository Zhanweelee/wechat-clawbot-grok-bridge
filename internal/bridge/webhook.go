package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Zhanweelee/wechat-clawbot-grok-bridge/internal/config"
	"github.com/Zhanweelee/wechat-clawbot-grok-bridge/internal/ilink"
)

// InboundWebhookPayload is sent to a Grok Bot webhook routine.
type InboundWebhookPayload struct {
	Source       string `json:"source"`
	FromUserID   string `json:"from_user_id"`
	ToUserID     string `json:"to_user_id,omitempty"`
	Text         string `json:"text"`
	ContextToken string `json:"context_token"`
	MessageID    string `json:"message_id,omitempty"`
	ReceivedAt   string `json:"received_at"`
}

// WebhookForwarder POSTs inbound WeChat messages to Grok.
type WebhookForwarder struct {
	URL     string
	Auth    string
	Client  *http.Client
}

// NewWebhookForwarder from config.
func NewWebhookForwarder(cfg config.Config) *WebhookForwarder {
	return &WebhookForwarder{
		URL:    cfg.WebhookURL,
		Auth:   cfg.WebhookAuthHeader(),
		Client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Forward sends one inbound user message to the configured webhook.
func (w *WebhookForwarder) Forward(ctx context.Context, msg ilink.WeixinMessage) error {
	if w.URL == "" {
		return fmt.Errorf("webhook_url is not configured")
	}
	text := ilink.ExtractPlainText(msg)
	if text == "" {
		return nil
	}
	body := InboundWebhookPayload{
		Source:       "wechat-clawbot-grok-bridge",
		FromUserID:   msg.FromUserID,
		ToUserID:     msg.ToUserID,
		Text:         text,
		ContextToken: msg.ContextToken,
		MessageID:    msg.MessageID,
		ReceivedAt:   time.Now().UTC().Format(time.RFC3339),
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(body); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.URL, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if w.Auth != "" {
		req.Header.Set("Authorization", w.Auth)
	}
	resp, err := w.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned HTTP %d", resp.StatusCode)
	}
	return nil
}
