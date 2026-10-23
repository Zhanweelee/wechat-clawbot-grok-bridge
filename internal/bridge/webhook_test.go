package bridge

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Zhanweelee/wechat-clawbot-grok-bridge/internal/config"
	"github.com/Zhanweelee/wechat-clawbot-grok-bridge/internal/ilink"
)

func TestWebhookForwarder(t *testing.T) {
	var got InboundWebhookPayload
	var auth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := config.Default()
	cfg.WebhookURL = srv.URL
	cfg.WebhookAuthorization = "Bearer test-key"
	f := NewWebhookForwarder(cfg)
	err := f.Forward(context.Background(), ilink.WeixinMessage{
		FromUserID:   "u@im.wechat",
		ContextToken: "ctx",
		ItemList: []ilink.MessageItem{{Type: 1, TextItem: &ilink.TextItem{Text: "hello"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "hello" || got.ContextToken != "ctx" {
		t.Fatalf("%+v", got)
	}
	if auth != "Bearer test-key" {
		t.Fatal(auth)
	}
}

func TestReplyHandlerAuth(t *testing.T) {
	cfg := config.Default()
	cfg.WebhookAuthorization = "Bearer key"
	b := &Bridge{Config: cfg, Client: ilink.NewClient(&ilink.Session{BotToken: "t", BaseURL: "http://127.0.0.1:9"})}

	req := httptest.NewRequest(http.MethodPost, "/v1/reply", io.NopCloser(nil))
	rec := httptest.NewRecorder()
	b.handleReply(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code %d", rec.Code)
	}
}
