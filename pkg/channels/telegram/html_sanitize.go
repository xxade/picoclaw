package telegram

import (
	"regexp"
	"strings"
)

// Incoming-HTML cleanup.
//
// LLMs increasingly emit HTML blocks (details/summary, b, br...) that neither
// Telegram parse mode understands as-is. Before markdown conversion we fold
// the useful bits into markdown and strip the rest so tags never leak to the
// user as literal "<...>" text.

var (
	reDetailsBlock = regexp.MustCompile(`(?is)<details[^>]*>\s*(?:<summary[^>]*>(.*?)</summary>)?\s*(.*?)\s*</details>`)
	reStrayHTMLTag = regexp.MustCompile(`</?(?:b|strong|i|em|u|ins|s|strike|del|span|div|p|br|hr|a|img|ul|ol|li|code|pre|kbd|samp|var|mark|small|big|font|center|blockquote|table|thead|tbody|tr|th|td|h[1-6])\b[^>]*/?>`)
)

// foldDetailsBlocks converts <details><summary>T</summary>BODY</details> into
// "**T**" followed by a markdown block quote of BODY — both understood
// natively by every Telegram renderer.
func foldDetailsBlocks(text string) string {
	return reDetailsBlock.ReplaceAllStringFunc(text, func(m string) string {
		sub := reDetailsBlock.FindStringSubmatch(m)
		title, body := "", sub[1]
		if len(sub) > 2 {
			title, body = strings.TrimSpace(sub[1]), strings.TrimSpace(sub[2])
		}
		var b strings.Builder
		if title != "" {
			b.WriteString("**▸ " + title + "**")
		}
		if body != "" {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			for _, ln := range strings.Split(body, "\n") {
				b.WriteString("> " + ln + "\n")
			}
		}
		return strings.TrimRight(b.String(), "\n")
	})
}

// stripStrayHTMLTags removes inline HTML markup outside of fenced code blocks,
// keeping human-readable content. Called after code blocks are already safe.
func stripStrayHTMLTags(text string) string {
	lines := strings.Split(text, "\n")
	inFence := false
	for i, ln := range lines {
		if isFenceLine(ln) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		lines[i] = reStrayHTMLTag.ReplaceAllString(ln, "")
	}
	return strings.Join(lines, "\n")
}
