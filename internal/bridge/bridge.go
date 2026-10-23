package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Zhanweelee/wechat-clawbot-grok-bridge/internal/config"
	"github.com/Zhanweelee/wechat-clawbot-grok-bridge/internal/ilink"
	"github.com/Zhanweelee/wechat-clawbot-grok-bridge/internal/session"
)

// Bridge runs long-poll loop, webhook forwarding, and optional reply HTTP API.
type Bridge struct {
	Config    config.Config
	Client    *ilink.Client
	Forwarder *WebhookForwarder
	Logger    *log.Logger
}

// ReplyRequest is accepted on the local HTTP API for Grok-driven replies.
type ReplyRequest struct {
	ToUserID      string `json:"to_user_id"`
	ContextToken  string `json:"context_token"`
	Text          string `json:"text"`
	ImageURL      string `json:"image_url,omitempty"`
}

// Run starts the bridge until interrupted.
func (b *Bridge) Run(ctx context.Context) error {
	if b.Logger == nil {
		b.Logger = log.New(os.Stdout, "", log.LstdFlags)
	}
	if err := b.Config.EnsureStateDir(); err != nil {
		return err
	}
	sessStore := session.Store{Path: b.Config.SessionPath()}
	sess, err := sessStore.Load()
	if err != nil {
		return fmt.Errorf("load session (run `login` first): %w", err)
	}
	b.Client = ilink.NewClient(sess)
	b.Client.RouteTag = b.Config.RouteTag
	if b.Forwarder == nil {
		b.Forwarder = NewWebhookForwarder(b.Config)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-sigCh:
			cancel()
		case <-ctx.Done():
		}
	}()

	if err := b.Client.NotifyStart(ctx); err != nil {
		b.Logger.Printf("warn: notifystart: %v", err)
	}
	defer func() {
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer stopCancel()
		if err := b.Client.NotifyStop(stopCtx); err != nil {
			b.Logger.Printf("warn: notifystop: %v", err)
		}
	}()

	var wg sync.WaitGroup
	errCh := make(chan error, 2)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := b.serveHTTP(ctx); err != nil && !errors.Is(err, context.Canceled) {
			errCh <- err
			cancel()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := b.pollLoop(ctx); err != nil && !errors.Is(err, context.Canceled) {
			errCh <- err
			cancel()
		}
	}()

	wg.Wait()
	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

func (b *Bridge) pollLoop(ctx context.Context) error {
	cursor, _ := session.LoadCursor(b.Config.CursorPath())
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		resp, err := b.Client.GetUpdates(ctx, cursor)
		if err != nil {
			b.Logger.Printf("getupdates error: %v; retry in 3s", err)
			if sleep(ctx, 3*time.Second) {
				return ctx.Err()
			}
			continue
		}
		cursor = resp.GetUpdatesBuf
		_ = session.SaveCursor(b.Config.CursorPath(), cursor)

		for _, msg := range resp.Msgs {
			if msg.MessageType == 2 {
				continue // skip bot-originated
			}
			if b.Config.EnableTypingIndicator && msg.FromUserID != "" && msg.ContextToken != "" {
				b.showTyping(ctx, msg.FromUserID, msg.ContextToken)
			}
			if err := b.Forwarder.Forward(ctx, msg); err != nil {
				b.Logger.Printf("webhook forward error: %v", err)
			}
		}

		timeout := resp.LongPollingTimeoutMS
		if timeout <= 0 {
			timeout = b.Config.LongPollTimeoutMS
		}
		_ = timeout // server holds connection; no client sleep needed
	}
}

func (b *Bridge) showTyping(ctx context.Context, userID, contextToken string) {
	ticket, err := b.Client.GetTypingTicket(ctx, userID, contextToken)
	if err != nil {
		b.Logger.Printf("typing ticket: %v", err)
		return
	}
	if err := b.Client.SendTyping(ctx, userID, ticket, 1); err != nil {
		b.Logger.Printf("sendtyping: %v", err)
	}
}

func (b *Bridge) serveHTTP(ctx context.Context) error {
	mux := http.NewServeMux()
	path := b.Config.ReplyHTTPPath
	if path == "" {
		path = "/v1/reply"
	}
	mux.HandleFunc(path, b.handleReply)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	srv := &http.Server{Addr: b.Config.ListenAddr, Handler: mux}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	b.Logger.Printf("reply API listening on http://%s%s", b.Config.ListenAddr, path)
	err := srv.ListenAndServe()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func (b *Bridge) handleReply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if auth := b.Config.WebhookAuthHeader(); auth != "" {
		if r.Header.Get("Authorization") != auth {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}
	defer r.Body.Close()
	var req ReplyRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if req.ToUserID == "" || req.ContextToken == "" {
		http.Error(w, "to_user_id and context_token required", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	if req.Text != "" {
		if err := b.Client.SendText(ctx, req.ToUserID, req.ContextToken, req.Text); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
	}
	if req.ImageURL != "" {
		if err := b.Client.SendImageURL(ctx, req.ToUserID, req.ContextToken, req.ImageURL); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func sleep(ctx context.Context, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return true
	case <-t.C:
		return false
	}
}
