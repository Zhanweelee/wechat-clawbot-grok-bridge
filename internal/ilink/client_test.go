package ilink

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateQRAndPoll(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ilink/bot/get_bot_qrcode", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method %s", r.Method)
		}
		_ = json.NewEncoder(w).Encode(QRCreateResponse{
			QRCode:           "qrc_test",
			QRCodeImgContent: "https://example.com/qr",
		})
	})
	mux.HandleFunc("/ilink/bot/get_qrcode_status", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("qrcode") != "qrc_test" {
			t.Fatalf("qrcode query")
		}
		_ = json.NewEncoder(w).Encode(QRStatusResponse{Status: "wait"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Point default API to test server by temporarily overriding via client hack:
	// CreateQR always uses DefaultAPIBase - for test we patch by using same host in Poll only.
	// CreateQR test uses real path on mock - we need mock at ilink host. Use srv.URL as override in test client.

	c := NewClient(nil)
	c.LoginAPIBase = srv.URL
	c.HTTP = srv.Client()

	qr, err := c.CreateQR(context.Background(), nil)
	if err != nil {
		t.Fatalf("create qr: %v", err)
	}
	if qr.QRCode != "qrc_test" {
		t.Fatalf("qrcode %q", qr.QRCode)
	}

	st, err := c.PollQRStatus(context.Background(), "qrc_test", "", srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if st.Status != "wait" {
		t.Fatalf("status %s", st.Status)
	}
}

func TestExtractPlainText(t *testing.T) {
	msg := WeixinMessage{
		ItemList: []MessageItem{
			{Type: 1, TextItem: &TextItem{Text: "hello"}},
			{Type: 1, TextItem: &TextItem{Text: "world"}},
		},
	}
	if ExtractPlainText(msg) != "hello\nworld" {
		t.Fatal(ExtractPlainText(msg))
	}
}

func TestGetUpdatesMock(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/ilink/bot/getupdates", func(w http.ResponseWriter, r *http.Request) {
		var req GetUpdatesRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.GetUpdatesBuf != "cursor1" {
			t.Fatalf("buf %q", req.GetUpdatesBuf)
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			t.Fatalf("auth")
		}
		_ = json.NewEncoder(w).Encode(GetUpdatesResponse{
			Ret:           0,
			GetUpdatesBuf: "cursor2",
			Msgs: []WeixinMessage{{
				FromUserID:   "user@im.wechat",
				ContextToken: "ctx",
				ItemList:     []MessageItem{{Type: 1, TextItem: &TextItem{Text: "hi"}}},
			}},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := NewClient(&Session{BotToken: "tok", BaseURL: srv.URL})
	resp, err := c.GetUpdates(context.Background(), "cursor1")
	if err != nil {
		t.Fatal(err)
	}
	if resp.GetUpdatesBuf != "cursor2" || len(resp.Msgs) != 1 {
		t.Fatalf("resp %+v", resp)
	}
}
