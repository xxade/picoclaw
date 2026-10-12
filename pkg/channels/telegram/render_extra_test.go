package telegram

import (
	"strings"
	"testing"
)

func TestFoldDetailsBlocks(t *testing.T) {
	src := "<details><summary>Показать распиновку</summary>DE — GPIO, RE — GPIO, TX/RX — FDCAN</details>"
	got := foldDetailsBlocks(src)
	if !strings.Contains(got, "**▸ Показать распиновку**") {
		t.Fatalf("summary lost: %q", got)
	}
	if !strings.Contains(got, "> DE — GPIO") {
		t.Fatalf("body not quoted: %q", got)
	}
	if strings.Contains(got, "<") {
		t.Fatalf("tags leaked: %q", got)
	}
}

func TestStripStrayHTMLTagsKeepsCode(t *testing.T) {
	src := "текст <b>жирный</b> и <br>перенос\n```\n<b>not touched</b>\n```"
	got := stripStrayHTMLTags(foldDetailsBlocks(src))
	if strings.Contains(got, "<b>жирный") || strings.Contains(got, "<br>") {
		t.Fatalf("tags not stripped: %q", got)
	}
	if !strings.Contains(got, "<b>not touched</b>") {
		t.Fatalf("code block modified: %q", got)
	}
}

func TestParseContentTaskAndItalic(t *testing.T) {
	src := "- [x] готово\n- [ ] нет\n*курсив* обычный"
	got := parseContent(src, false)
	if !strings.Contains(got, "✅") || !strings.Contains(got, "⬜") {
		t.Fatalf("task list not folded: %q", got)
	}
	if !strings.Contains(got, "<i>курсив</i>") {
		t.Fatalf("star italic missing: %q", got)
	}
}

func TestParseContentBlockquoteAllLines(t *testing.T) {
	src := "> первая\n> вторая."
	got := parseContent(src, false)
	if strings.Contains(got, "&gt;") {
		t.Fatalf("blockquote leaked on some line: %q", got)
	}
}
