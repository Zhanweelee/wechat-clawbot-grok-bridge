package login

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Zhanweelee/wechat-clawbot-grok-bridge/internal/config"
	"github.com/Zhanweelee/wechat-clawbot-grok-bridge/internal/ilink"
	"github.com/Zhanweelee/wechat-clawbot-grok-bridge/internal/session"
)

// Options for interactive QR login.
type Options struct {
	Config       config.Config
	VerifyCode   string
	PollInterval time.Duration
	Out          func(string)
}

// Run performs QR login and saves session credentials.
func Run(ctx context.Context, opt Options) (*ilink.Session, error) {
	if opt.PollInterval <= 0 {
		opt.PollInterval = 2 * time.Second
	}
	out := opt.Out
	if out == nil {
		out = func(s string) { fmt.Fprintln(os.Stdout, s) }
	}
	if err := opt.Config.EnsureStateDir(); err != nil {
		return nil, err
	}

	var localTokens []string
	sessStore := session.Store{Path: opt.Config.SessionPath()}
	if existing, err := sessStore.Load(); err == nil && existing.BotToken != "" {
		localTokens = []string{existing.BotToken}
	}

	client := ilink.NewClient(nil)
	client.RouteTag = opt.Config.RouteTag

	qr, err := client.CreateQR(ctx, localTokens)
	if err != nil {
		return nil, err
	}
	out(fmt.Sprintf("Scan this WeChat QR (ClawBot / iLink binding):\n  %s", qr.QRCodeImgContent))
	out(fmt.Sprintf("Polling token: %s", qr.QRCode))

	statusHost := ""
	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		st, err := client.PollQRStatus(ctx, qr.QRCode, opt.VerifyCode, statusHost)
		if err != nil {
			out(fmt.Sprintf("poll error: %v", err))
			if sleep(ctx, opt.PollInterval) {
				return nil, ctx.Err()
			}
			continue
		}
		switch st.Status {
		case "wait", "scaned":
			out("status: " + st.Status + " — waiting…")
		case "need_verifycode":
			out("status: need_verifycode — enter the code shown on your phone with --verify-code")
			return nil, fmt.Errorf("verification code required")
		case "verify_code_blocked":
			return nil, fmt.Errorf("verification blocked; refresh QR with login again")
		case "expired":
			return nil, fmt.Errorf("QR expired; run login again")
		case "scaned_but_redirect":
			if st.RedirectHost != "" {
				statusHost = st.RedirectHost
				out("redirecting poll host to " + statusHost)
			}
		case "binded_redirect":
			out("bot already bound — continuing")
		case "confirmed":
			sess := &ilink.Session{
				BotToken:    st.BotToken,
				BaseURL:     st.BaseURL,
				ILinkBotID:  st.ILinkBotID,
				ILinkUserID: st.ILinkUserID,
			}
			if sess.BaseURL == "" {
				sess.BaseURL = ilink.DefaultAPIBase
			}
			if err := sessStore.Save(sess); err != nil {
				return nil, err
			}
			out("login confirmed — session saved to " + opt.Config.SessionPath())
			return sess, nil
		default:
			out("status: " + st.Status)
		}
		if sleep(ctx, opt.PollInterval) {
			return nil, ctx.Err()
		}
	}
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
