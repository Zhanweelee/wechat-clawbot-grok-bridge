package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/Zhanweelee/wechat-clawbot-grok-bridge/internal/bridge"
	"github.com/Zhanweelee/wechat-clawbot-grok-bridge/internal/config"
	"github.com/Zhanweelee/wechat-clawbot-grok-bridge/internal/ilink"
	"github.com/Zhanweelee/wechat-clawbot-grok-bridge/internal/login"
	"github.com/Zhanweelee/wechat-clawbot-grok-bridge/internal/session"
	"github.com/spf13/cobra"
)

var (
	cfgPath     string
	verifyCode  string
	toUser      string
	contextTok  string
	messageText string
	imageURL    string
)

func main() {
	root := &cobra.Command{
		Use:   "wechat-clawbot-grok-bridge",
		Short: "WeChat ClawBot (iLink) ↔ Grok Bot webhook bridge",
	}
	root.PersistentFlags().StringVar(&cfgPath, "config", "", "Path to config.json (default: ~/.wechat-clawbot-grok-bridge/config.json)")

	root.AddCommand(
		loginCmd(),
		serveCmd(),
		sendCmd(),
		statusCmd(),
	)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func loadCfg() (config.Config, error) {
	path := cfgPath
	if path == "" {
		def := config.Default()
		path = def.StateDir + "/config.json"
	}
	if _, err := os.Stat(path); err != nil {
		cfg := config.Default()
		_ = cfg.EnsureStateDir()
		return cfg, nil
	}
	return config.Load(path)
}

func loginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "QR login / bind WeChat ClawBot via iLink",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadCfg()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			_, err = login.Run(ctx, login.Options{
				Config:       cfg,
				VerifyCode:   verifyCode,
				PollInterval: 2 * time.Second,
			})
			return err
		},
	}
	cmd.Flags().StringVar(&verifyCode, "verify-code", "", "Verification code if WeChat prompts for one")
	return cmd
}

func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start long-poll bridge (forward inbound messages to Grok webhook)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadCfg()
			if err != nil {
				return err
			}
			if cfg.WebhookURL == "" {
				return fmt.Errorf("webhook_url is required (config or WCG_WEBHOOK_URL)")
			}
			b := &bridge.Bridge{Config: cfg}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			return b.Run(ctx)
		},
	}
}

func sendCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send an outbound text or image message",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadCfg()
			if err != nil {
				return err
			}
			sessStore := session.Store{Path: cfg.SessionPath()}
			sess, err := sessStore.Load()
			if err != nil {
				return fmt.Errorf("no session: run login first: %w", err)
			}
			if toUser == "" || contextTok == "" {
				return fmt.Errorf("--to and --context-token are required")
			}
			if messageText == "" && imageURL == "" {
				return fmt.Errorf("--text or --image-url is required")
			}
			client := ilink.NewClient(sess)
			client.RouteTag = cfg.RouteTag
			ctx := context.Background()
			if messageText != "" {
				if err := client.SendText(ctx, toUser, contextTok, messageText); err != nil {
					return err
				}
			}
			if imageURL != "" {
				if err := client.SendImageURL(ctx, toUser, contextTok, imageURL); err != nil {
					return err
				}
			}
			fmt.Println("sent")
			return nil
		},
	}
	cmd.Flags().StringVar(&toUser, "to", "", "Target iLink user id")
	cmd.Flags().StringVar(&contextTok, "context-token", "", "Conversation context_token from inbound message")
	cmd.Flags().StringVar(&messageText, "text", "", "Text body")
	cmd.Flags().StringVar(&imageURL, "image-url", "", "Public image URL (best-effort)")
	return cmd
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show login and configuration status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadCfg()
			if err != nil {
				return err
			}
			fmt.Println("state_dir:", cfg.StateDir)
			fmt.Println("webhook_url:", mask(cfg.WebhookURL))
			fmt.Println("webhook_auth:", mask(cfg.WebhookAuthHeader()))
			fmt.Println("listen_addr:", cfg.ListenAddr)
			sessStore := session.Store{Path: cfg.SessionPath()}
			sess, err := sessStore.Load()
			if err != nil {
				fmt.Println("session: not logged in")
				return nil
			}
			fmt.Println("session: ok")
			fmt.Println("  ilink_bot_id:", sess.ILinkBotID)
			fmt.Println("  ilink_user_id:", sess.ILinkUserID)
			fmt.Println("  baseurl:", sess.BaseURL)
			fmt.Println("  saved_at:", sess.SavedAt)
			return nil
		},
	}
}

func mask(s string) string {
	if s == "" {
		return "(not set)"
	}
	if len(s) <= 8 {
		return "***"
	}
	return s[:4] + "…" + s[len(s)-4:]
}
