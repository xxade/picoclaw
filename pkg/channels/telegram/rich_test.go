package telegram

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	ta "github.com/mymmrac/telego/telegoapi"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func richTestChannel(t *testing.T, caller *stubCaller) *TelegramChannel {
	t.Helper()
	ch := newTestChannel(t, caller)
	richOn := true
	ch.tgCfg.RichFormat = &richOn
	return ch
}

func okResponse(msgID int) *ta.Response {
	res, _ := json.Marshal(map[string]any{"message_id": msgID, "date": 1, "chat": map[string]any{"id": 1, "type": "private"}})
	return &ta.Response{Ok: true, Result: res}
}

func richRejectResponse(desc string) *ta.Response {
	return &ta.Response{Ok: false, Error: &ta.Error{ErrorCode: 400, Description: desc}}
}

func TestSendUsesRichByDefault(t *testing.T) {
	var urls []string
	ch := richTestChannel(t, &stubCaller{
		callFn: func(_ context.Context, u string, _ *ta.RequestData) (*ta.Response, error) {
			urls = append(urls, u)
			return okResponse(10), nil
		},
	})
	ids, err := ch.Send(context.Background(), bus.OutboundMessage{
		ChatID:  "1",
		Content: "| A | B |\n|---|---|\n| 1 | 2 |",
	})
	require.NoError(t, err)
	require.Len(t, urls, 1)
	assert.Contains(t, urls[0], "sendRichMessage")
	assert.Equal(t, []string{"10"}, ids)
}

func TestSendRichParsesMarkdownRaw(t *testing.T) {
	var sent string
	ch := richTestChannel(t, &stubCaller{
		callFn: func(_ context.Context, _ string, data *ta.RequestData) (*ta.Response, error) {
			var req struct {
				RichMessage struct {
					Markdown string `json:"markdown"`
				} `json:"rich_message"`
			}
			_ = json.Unmarshal(data.BodyRaw, &req)
			sent = req.RichMessage.Markdown
			return okResponse(1), nil
		},
	})
	src := "| A | B |\n|---|---|\n| 1 | 2 |\n- [x] ok"
	_, err := ch.Send(context.Background(), bus.OutboundMessage{ChatID: "1", Content: src})
	require.NoError(t, err)
	assert.Equal(t, src, sent, "rich path must forward raw markdown untouched")
}

func TestSendRichFallbackOnReject(t *testing.T) {
	var urls []string
	ch := richTestChannel(t, &stubCaller{
		callFn: func(_ context.Context, u string, _ *ta.RequestData) (*ta.Response, error) {
			urls = append(urls, u)
			if strings.Contains(u, "sendRichMessage") {
				return richRejectResponse("Bad Request: RICH_MESSAGE_TABLE_ROWS_TOO_MANY"), nil
			}
			return okResponse(2), nil
		},
	})
	_, err := ch.Send(context.Background(), bus.OutboundMessage{ChatID: "1", Content: "text **bold**"})
	require.NoError(t, err)
	require.Len(t, urls, 2)
	assert.Contains(t, urls[0], "sendRichMessage")
	assert.Contains(t, urls[1], "sendMessage")
}

func TestSendWideTableSkipsRich(t *testing.T) {
	var urls []string
	ch := richTestChannel(t, &stubCaller{
		callFn: func(_ context.Context, u string, _ *ta.RequestData) (*ta.Response, error) {
			urls = append(urls, u)
			return okResponse(1), nil
		},
	})
	hdr := "| " + strings.Join(repeatN("C", 21), " | ") + " |"
	sep := "|" + strings.Repeat("---|", 21)
	_, err := ch.Send(context.Background(), bus.OutboundMessage{
		ChatID: "1", Content: hdr + "\n" + sep + "\n" + hdr,
	})
	require.NoError(t, err)
	require.NotEmpty(t, urls)
	assert.NotContains(t, urls[0], "sendRichMessage", "21-column table must skip the rich attempt")
}

func TestSendRichDisabledUsesLegacy(t *testing.T) {
	var urls []string
	ch := newTestChannel(t, &stubCaller{
		callFn: func(_ context.Context, u string, _ *ta.RequestData) (*ta.Response, error) {
			urls = append(urls, u)
			return okResponse(1), nil
		},
	})
	_, err := ch.Send(context.Background(), bus.OutboundMessage{ChatID: "1", Content: "**x**"})
	require.NoError(t, err)
	require.Len(t, urls, 1)
	assert.Contains(t, urls[0], "sendMessage")
}

func TestRichFormatEnabledDefault(t *testing.T) {
	var nilCfg *config.TelegramSettings
	assert.True(t, nilCfg.RichFormatEnabled())
	assert.True(t, (&config.TelegramSettings{}).RichFormatEnabled())
	off := false
	assert.False(t, (&config.TelegramSettings{RichFormat: &off}).RichFormatEnabled())
}

func TestRichParseFailureClassification(t *testing.T) {
	assert.True(t, richParseFailure(errStr("telego: sendRichMessage: ... Bad Request: RICH_MESSAGE_TABLE_COLS_TOO_MANY")))
	assert.True(t, richParseFailure(errStr("api: 400 \"Bad Request: rich message must be non-empty\"")))
	assert.False(t, richParseFailure(errStr("fasthttp do request: lookup api.telegram.org: i/o timeout")))
	assert.False(t, richParseFailure(nil))
}

type errStr string

func (e errStr) Error() string { return string(e) }

func repeatN(s string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = s + itoaTest(i)
	}
	return out
}

func itoaTest(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for n > 0 {
		d = append([]byte{byte('0' + n%10)}, d...)
		n /= 10
	}
	return string(d)
}
