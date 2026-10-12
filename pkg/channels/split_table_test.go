package channels

import (
	"strings"
	"testing"
)

func tableOf(n int, label string) string {
	var b strings.Builder
	b.WriteString("| Сыр | Вес | Цена |\n|:---|---:|---:|\n")
	for i := 1; i <= n; i++ {
		b.WriteString("| " + label + " | 350 г | 209,99 |\n")
	}
	return b.String()
}

// everyChunkWithRowsHasHeader checks that each chunk containing pipe-table
// rows also carries a header+separator pair covering those rows: Telegram
// renders headerless rows as loose pipe text.
func everyChunkWithRowsHasHeader(text string) bool {
	lines := strings.Split(text, "\n")
	inFence := false
	coverage := make([]bool, len(lines))
	for i := 0; i+1 < len(lines); i++ {
		tr := strings.TrimSpace(lines[i])
		if strings.HasPrefix(tr, "```") {
			inFence = !inFence
		}
		if inFence {
			continue
		}
		if isTableRowLine(tr) && isTableSepLine(strings.TrimSpace(lines[i+1])) {
			coverage[i] = true
			coverage[i+1] = true
			for j := i + 2; j < len(lines); j++ {
				trj := strings.TrimSpace(lines[j])
				if !isTableRowLine(trj) {
					break
				}
				coverage[j] = true
			}
		}
	}
	for i, l := range lines {
		tr := strings.TrimSpace(l)
		if strings.HasPrefix(tr, "```") {
			continue
		}
		if isTableRowLine(tr) && !coverage[i] {
			return false
		}
	}
	return true
}

func TestSplitMessageKeepsTableIntact(t *testing.T) {
	prefix := strings.Repeat("вводный абзац текста для набивки. ", 60)
	text := prefix + "\n\n" + tableOf(120, "Экомилк 55%") + "\nВыводы под твои рецепты\n- пункт\n"
	chunks := SplitMessage(text, 4000)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if len([]rune(c)) > 4096 {
			t.Errorf("chunk %d exceeds telegram limit: %d runes", i, len([]rune(c)))
		}
		if !everyChunkWithRowsHasHeader(c) {
			t.Errorf("chunk %d has table rows without header", i)
		}
	}
	rows := 0
	for _, c := range chunks {
		rows += strings.Count(c, "| Экомилк 55% |")
	}
	if rows != 120 {
		t.Errorf("expected 120 table rows across chunks, got %d", rows)
	}
}

func TestSplitMessageRepeatsHeaderForHugeTable(t *testing.T) {
	text := tableOf(200, "Экомилк 55%")
	chunks := SplitMessage(text, 4000)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		if !everyChunkWithRowsHasHeader(c) {
			t.Errorf("chunk %d has table rows without header", i)
		}
	}
	rows := 0
	for _, c := range chunks {
		rows += strings.Count(c, "| Экомилк 55% |")
	}
	if rows != 200 {
		t.Errorf("expected all 200 rows preserved, got %d", rows)
	}
}

func TestSplitMessageDoesNotTreatPipeTextAsTable(t *testing.T) {
	text := "обычный текст | с pipe | внутри\n" + strings.Repeat("и ещё строка | две | три\n", 300)
	chunks := SplitMessage(text, 4000)
	for _, c := range chunks {
		if strings.Contains(c, "|:---|") {
			t.Errorf("header injected into non-table text:\n%s", c[:200])
		}
	}
}
