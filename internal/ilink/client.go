package ilink

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	DefaultAPIBase   = "https://ilinkai.weixin.qq.com"
	AppID            = "bot"
	ClientVersion    = "1"
	BotAgent         = "wechat-clawbot-grok-bridge"
	ChannelVersion   = "0.1.0"
)

// Client talks to Tencent iLink bot HTTP APIs.
type Client struct {
	HTTP        *http.Client
	APIBase     string
	LoginAPIBase string // optional override for QR endpoints (tests)
	Session     *Session
	RouteTag    string
	PluginVer   string
}

func (c *Client) loginBase() string {
	if c.LoginAPIBase != "" {
		return strings.TrimRight(c.LoginAPIBase, "/")
	}
	return DefaultAPIBase
}

// NewClient creates a client with optional session.
func NewClient(session *Session) *Client {
	base := DefaultAPIBase
	if session != nil && session.BaseURL != "" {
		base = strings.TrimRight(session.BaseURL, "/")
	}
	return &Client{
		HTTP: &http.Client{Timeout: 120 * time.Second},
		APIBase: base,
		Session: session,
		PluginVer: ChannelVersion,
	}
}

func (c *Client) baseInfo() BaseInfo {
	return BaseInfo{
		ChannelVersion: c.PluginVer,
		BotAgent:       BotAgent,
	}
}

func randomUINHeader() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	n := binary.BigEndian.Uint32(b[:])
	return base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d", n)))
}

func (c *Client) commonHeaders(auth bool, jsonBody bool) http.Header {
	h := http.Header{}
	h.Set("iLink-App-Id", AppID)
	h.Set("iLink-App-ClientVersion", ClientVersion)
	if c.RouteTag != "" {
		h.Set("SKRouteTag", c.RouteTag)
	}
	if jsonBody {
		h.Set("Content-Type", "application/json")
	}
	if auth {
		h.Set("AuthorizationType", "ilink_bot_token")
		h.Set("X-WECHAT-UIN", randomUINHeader())
		if c.Session != nil && c.Session.BotToken != "" {
			h.Set("Authorization", "Bearer "+c.Session.BotToken)
		}
	}
	return h
}

func (c *Client) postJSON(ctx context.Context, path string, body any, auth bool) ([]byte, int, error) {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return nil, 0, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.APIBase+path, &buf)
	if err != nil {
		return nil, 0, err
	}
	req.Header = c.commonHeaders(auth, body != nil)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return data, resp.StatusCode, fmt.Errorf("ilink HTTP %d: %s", resp.StatusCode, truncate(data, 256))
	}
	return data, resp.StatusCode, nil
}

func (c *Client) get(ctx context.Context, fullURL string, auth bool) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header = c.commonHeaders(auth, false)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return data, resp.StatusCode, fmt.Errorf("ilink HTTP %d: %s", resp.StatusCode, truncate(data, 256))
	}
	return data, resp.StatusCode, nil
}

func truncate(b []byte, n int) string {
	s := string(b)
	if len(s) > n {
		return s[:n] + "..."
	}
	return s
}

// CreateQR starts a bot QR login session at the fixed login host.
func (c *Client) CreateQR(ctx context.Context, localTokens []string) (*QRCreateResponse, error) {
	loginClient := *c
	loginClient.APIBase = c.loginBase()
	body := map[string]any{"local_token_list": localTokens}
	data, _, err := loginClient.postJSON(ctx, "/ilink/bot/get_bot_qrcode?bot_type=3", body, false)
	if err != nil {
		return nil, err
	}
	var out QRCreateResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	if out.QRCode == "" {
		return nil, fmt.Errorf("empty qrcode in response")
	}
	return &out, nil
}

// PollQRStatus polls login status; verifyCode optional for need_verifycode.
func (c *Client) PollQRStatus(ctx context.Context, qrcode string, verifyCode string, statusHost string) (*QRStatusResponse, error) {
	host := c.loginBase()
	if statusHost != "" {
		statusHost = strings.TrimRight(statusHost, "/")
		if strings.HasPrefix(statusHost, "http") {
			host = statusHost
		} else {
			host = "https://" + statusHost
		}
	}
	u, err := url.Parse(host + "/ilink/bot/get_qrcode_status")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("qrcode", qrcode)
	if verifyCode != "" {
		q.Set("verify_code", verifyCode)
	}
	u.RawQuery = q.Encode()

	loginClient := *c
	loginClient.APIBase = ""
	data, _, err := loginClient.get(ctx, u.String(), false)
	if err != nil {
		return nil, err
	}
	var out QRStatusResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// NotifyStart tells the backend the channel started.
func (c *Client) NotifyStart(ctx context.Context) error {
	return c.postLifecycle(ctx, "/ilink/bot/msg/notifystart")
}

// NotifyStop tells the backend the channel stopped.
func (c *Client) NotifyStop(ctx context.Context) error {
	return c.postLifecycle(ctx, "/ilink/bot/msg/notifystop")
}

func (c *Client) postLifecycle(ctx context.Context, path string) error {
	body := map[string]any{"base_info": c.baseInfo()}
	data, _, err := c.postJSON(ctx, path, body, true)
	if err != nil {
		return err
	}
	var resp APIResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}
	if resp.Ret != 0 {
		return fmt.Errorf("lifecycle %s ret=%d: %s", path, resp.Ret, resp.ErrMsg)
	}
	return nil
}

// GetUpdates long-polls for inbound messages.
func (c *Client) GetUpdates(ctx context.Context, cursor string) (*GetUpdatesResponse, error) {
	req := GetUpdatesRequest{
		BaseInfo:      c.baseInfo(),
		GetUpdatesBuf: cursor,
	}
	data, _, err := c.postJSON(ctx, "/ilink/bot/getupdates", req, true)
	if err != nil {
		return nil, err
	}
	var out GetUpdatesResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	if out.Ret != 0 {
		return nil, fmt.Errorf("getupdates ret=%d errcode=%d: %s", out.Ret, out.ErrCode, out.ErrMsg)
	}
	return &out, nil
}

// SendText sends a text reply in a conversation.
func (c *Client) SendText(ctx context.Context, toUserID, contextToken, text string) error {
	req := SendMessageRequest{
		BaseInfo: c.baseInfo(),
		Msg: OutboundMessage{
			ToUserID:     toUserID,
			ContextToken: contextToken,
			ItemList: []MessageItem{{
				Type:     1,
				TextItem: &TextItem{Text: text},
			}},
		},
	}
	data, _, err := c.postJSON(ctx, "/ilink/bot/sendmessage", req, true)
	if err != nil {
		return err
	}
	var resp APIResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}
	if resp.Ret != 0 {
		return fmt.Errorf("sendmessage ret=%d: %s", resp.Ret, resp.ErrMsg)
	}
	return nil
}

// SendImageURL sends an image message using a public image URL (when supported by the API).
func (c *Client) SendImageURL(ctx context.Context, toUserID, contextToken, imageURL string) error {
	req := SendMessageRequest{
		BaseInfo: c.baseInfo(),
		Msg: OutboundMessage{
			ToUserID:     toUserID,
			ContextToken: contextToken,
			ItemList: []MessageItem{{
				Type: 2,
				ImageItem: &ImageItem{URL: imageURL},
			}},
		},
	}
	data, _, err := c.postJSON(ctx, "/ilink/bot/sendmessage", req, true)
	if err != nil {
		return err
	}
	var resp APIResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}
	if resp.Ret != 0 {
		return fmt.Errorf("sendmessage image ret=%d: %s", resp.Ret, resp.ErrMsg)
	}
	return nil
}

// GetTypingTicket fetches typing ticket for a conversation.
func (c *Client) GetTypingTicket(ctx context.Context, userID, contextToken string) (string, error) {
	req := GetConfigRequest{
		BaseInfo:     c.baseInfo(),
		ILinkUserID:  userID,
		ContextToken: contextToken,
	}
	data, _, err := c.postJSON(ctx, "/ilink/bot/getconfig", req, true)
	if err != nil {
		return "", err
	}
	var out GetConfigResponse
	if err := json.Unmarshal(data, &out); err != nil {
		return "", err
	}
	if out.Ret != 0 {
		return "", fmt.Errorf("getconfig ret=%d: %s", out.Ret, out.ErrMsg)
	}
	return out.TypingTicket, nil
}

// SendTyping sends or cancels typing indicator (status 1 or 2).
func (c *Client) SendTyping(ctx context.Context, userID, ticket string, status int) error {
	req := SendTypingRequest{
		BaseInfo:     c.baseInfo(),
		ILinkUserID:  userID,
		TypingTicket: ticket,
		Status:       status,
	}
	data, _, err := c.postJSON(ctx, "/ilink/bot/sendtyping", req, true)
	if err != nil {
		return err
	}
	var resp APIResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return err
	}
	if resp.Ret != 0 {
		return fmt.Errorf("sendtyping ret=%d: %s", resp.Ret, resp.ErrMsg)
	}
	return nil
}

// ExtractPlainText concatenates text items from a message.
func ExtractPlainText(msg WeixinMessage) string {
	var parts []string
	for _, item := range msg.ItemList {
		if item.Type == 1 && item.TextItem != nil && item.TextItem.Text != "" {
			parts = append(parts, item.TextItem.Text)
		}
	}
	return strings.Join(parts, "\n")
}
