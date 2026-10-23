package ilink

// Session holds credentials returned after QR login confirmation.
type Session struct {
	BotToken    string `json:"bot_token"`
	BaseURL     string `json:"baseurl"`
	ILinkBotID  string `json:"ilink_bot_id"`
	ILinkUserID string `json:"ilink_user_id"`
	SavedAt     string `json:"saved_at,omitempty"`
}

// QRCreateResponse is returned by get_bot_qrcode.
type QRCreateResponse struct {
	QRCode           string `json:"qrcode"`
	QRCodeImgContent string `json:"qrcode_img_content"`
}

// QRStatusResponse is returned by get_qrcode_status.
type QRStatusResponse struct {
	Status        string `json:"status"`
	BotToken      string `json:"bot_token"`
	BaseURL       string `json:"baseurl"`
	ILinkBotID    string `json:"ilink_bot_id"`
	ILinkUserID   string `json:"ilink_user_id"`
	RedirectHost  string `json:"redirect_host"`
	VerifyCode    string `json:"verify_code,omitempty"`
}

// BaseInfo is included on authenticated POST bodies.
type BaseInfo struct {
	ChannelVersion string `json:"channel_version"`
	BotAgent       string `json:"bot_agent"`
}

// GetUpdatesRequest long-poll request.
type GetUpdatesRequest struct {
	BaseInfo       BaseInfo `json:"base_info"`
	GetUpdatesBuf  string   `json:"get_updates_buf"`
}

// GetUpdatesResponse long-poll response.
type GetUpdatesResponse struct {
	Ret                  int            `json:"ret"`
	ErrCode              int            `json:"errcode"`
	ErrMsg               string         `json:"errmsg"`
	Msgs                 []WeixinMessage `json:"msgs"`
	GetUpdatesBuf        string         `json:"get_updates_buf"`
	LongPollingTimeoutMS int            `json:"longpolling_timeout_ms"`
}

// WeixinMessage inbound or outbound message envelope.
type WeixinMessage struct {
	Seq           int64         `json:"seq,omitempty"`
	MessageID     string        `json:"message_id,omitempty"`
	FromUserID    string        `json:"from_user_id,omitempty"`
	ToUserID      string        `json:"to_user_id,omitempty"`
	CreateTimeMS  int64         `json:"create_time_ms,omitempty"`
	MessageType   int           `json:"message_type,omitempty"`
	MessageState  int           `json:"message_state,omitempty"`
	ItemList      []MessageItem `json:"item_list,omitempty"`
	ContextToken  string        `json:"context_token,omitempty"`
}

// MessageItem content block inside a message.
type MessageItem struct {
	Type      int        `json:"type"`
	TextItem  *TextItem  `json:"text_item,omitempty"`
	ImageItem *ImageItem `json:"image_item,omitempty"`
}

// TextItem type=1 text payload.
type TextItem struct {
	Text string `json:"text"`
}

// ImageItem type=2 image reference (outbound may use URL).
type ImageItem struct {
	URL string `json:"url,omitempty"`
}

// SendMessageRequest outbound message.
type SendMessageRequest struct {
	BaseInfo BaseInfo       `json:"base_info"`
	Msg      OutboundMessage `json:"msg"`
}

type OutboundMessage struct {
	ToUserID      string        `json:"to_user_id"`
	ContextToken  string        `json:"context_token"`
	ItemList      []MessageItem `json:"item_list"`
}

// APIResponse generic ret/errmsg wrapper.
type APIResponse struct {
	Ret    int    `json:"ret"`
	ErrMsg string `json:"errmsg"`
}

// GetConfigRequest typing ticket request.
type GetConfigRequest struct {
	BaseInfo      BaseInfo `json:"base_info"`
	ILinkUserID   string   `json:"ilink_user_id"`
	ContextToken  string   `json:"context_token,omitempty"`
}

// GetConfigResponse typing ticket response.
type GetConfigResponse struct {
	Ret          int    `json:"ret"`
	ErrMsg       string `json:"errmsg"`
	TypingTicket string `json:"typing_ticket"`
}

// SendTypingRequest typing indicator.
type SendTypingRequest struct {
	BaseInfo      BaseInfo `json:"base_info"`
	ILinkUserID   string   `json:"ilink_user_id"`
	TypingTicket  string   `json:"typing_ticket"`
	Status        int      `json:"status"` // 1 typing, 2 cancel
}
