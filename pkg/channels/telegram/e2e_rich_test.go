package telegram

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/sipeed/picoclaw/pkg/bus"
	"github.com/sipeed/picoclaw/pkg/config"
)

// Real-API e2e: run the channel Send() against Telegram production with the
// user's test bot. Only runs when TG_E2E_TOKEN is set.
func TestE2ESendRichViaChannel(t *testing.T) {
	tok := os.Getenv("TG_E2E_TOKEN")
	chat := os.Getenv("TG_E2E_CHAT")
	if tok == "" || chat == "" {
		t.Skip("TG_E2E_TOKEN/TG_E2E_CHAT not set")
	}
	fix, err := os.ReadFile("testdata/can_cheatsheet.md")
	if err != nil {
		t.Fatal(err)
	}

	b := config.NewSecureString(tok)
	ch, err := NewTelegramChannel(
		&config.Channel{Type: config.ChannelTelegram, Enabled: true, AllowFrom: []string{chat}},
		&config.TelegramSettings{Token: *b},
		bus.NewMessageBus(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := ch.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer ch.Stop(context.Background())

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ids, err := ch.Send(ctx, bus.OutboundMessage{ChatID: chat, Content: string(fix)})
	if err != nil {
		t.Fatalf("Send failed: %v", err)
	}
	t.Logf("sent message IDs: %v", ids)
}
