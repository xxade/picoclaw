package telegram

import (
	"context"
	"strconv"
	"strings"

	"github.com/mymmrac/telego"
	tu "github.com/mymmrac/telego/telegoutil"
)

// Telegram "Rich messages" (Bot API 10.1+, July 2026): the bot sends content
// through sendRichMessage and clients render real blocks — tables, headings,
// checklists, collapsible <details>, quotes — natively. See
// https://core.telegram.org/bots/api#rich-message-formatting-options
//
// Strategy: rich is the preferred renderer for LLM markdown. Anything the
// rich parser rejects (or that exceeds rich limits, e.g. tables with more
// than 20 columns) falls back to the legacy HTML/MarkdownV2 pipeline, which
// itself degrades to plain text on parse failure.

// richTableColLimit is the hard Bot API limit for table columns
// (RICH_MESSAGE_TABLE_COLS_TOO_MANY).
const richTableColLimit = 20

// richMaxMessageRunes mirrors the documented rich message size limit.
const richMaxMessageRunes = 32768

// richContentUnsupported reports whether the markdown is known up-front to
// violate rich message limits, most importantly tables wider than 20
// columns. Such content should skip the rich attempt and go straight to the
// legacy renderer.
func richContentUnsupported(text string) bool {
	lines := strings.Split(text, "\n")
	for _, tb := range detectTablesInLines(lines) {
		if len(splitTableCells(lines[tb.start])) > richTableColLimit {
			return true
		}
	}
	if len([]rune(text)) > richMaxMessageRunes {
		return true
	}
	return false
}

// sendRichText sends one text message through sendRichMessage with raw
// markdown. Telegram parses standard GFM markdown natively (tables, task
// lists, headings, block quotes, <details>...), so no conversion is applied.
func (c *TelegramChannel) sendRichText(
	ctx context.Context,
	chatID int64,
	threadID int,
	text string,
	replyToID string,
) (int, error) {
	params := &telego.SendRichMessageParams{
		ChatID:          tu.ID(chatID),
		MessageThreadID: threadID,
		RichMessage: telego.InputRichMessage{
			Markdown: text,
		},
	}
	if replyToID != "" {
		if mid, err := strconv.Atoi(replyToID); err == nil {
			params.ReplyParameters = &telego.ReplyParameters{MessageID: mid}
		}
	}
	msg, err := c.bot.SendRichMessage(ctx, params)
	if err != nil {
		return 0, err
	}
	return msg.MessageID, nil
}

// richEnabled reports whether the rich renderer may be used for this channel.
func (c *TelegramChannel) richEnabled() bool {
	return c != nil && c.tgCfg != nil && c.tgCfg.RichFormatEnabled()
}

// editRichMessage replaces a message's content with a new rich message.
func (c *TelegramChannel) editRichMessage(ctx context.Context, chatID int64, messageID int, text string) error {
	_, err := c.bot.EditMessageText(ctx, &telego.EditMessageTextParams{
		ChatID:    tu.ID(chatID),
		MessageID: messageID,
		RichMessage: &telego.InputRichMessage{
			Markdown: text,
		},
	})
	return err
}

// richParseFailure reports whether err looks like Telegram rejecting the
// rich message content (as opposed to a network/permission problem, where a
// legacy retry would fail too or shouldn't be attempted at all).
func richParseFailure(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if !strings.Contains(msg, "Bad Request") {
		return false
	}
	for _, marker := range []string{"RICH_MESSAGE", "rich message"} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return false
}
