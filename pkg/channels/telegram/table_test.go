package telegram

import (
	"strings"
	"testing"
)

func TestParseGFMTableBasic(t *testing.T) {
	src := "| Режим | Скорость |\n|:---|---:|\n| CAN | 500 |\n| FD | 2000 |"
	got := renderTablesIntoMarkdown(src, tableModeCode, 60)
	if !strings.Contains(got, "┌") || !strings.Contains(got, "└") {
		t.Fatalf("expected box-drawing table, got:\n%s", got)
	}
	if !strings.Contains(got, "```") {
		t.Fatalf("expected fenced code block wrapper")
	}
	if strings.Contains(got, "Скорость |\n|:---") {
		t.Fatalf("raw table leaked into output")
	}
}

func TestParseTableEscapedPipeAndEmptyCell(t *testing.T) {
	src := "| A | B |\n|---|---|\n| x \\| y |  |\n| p | q |"
	lines := strings.Split(src, "\n")
	tables := detectTablesInLines(lines)
	if len(tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(tables))
	}
	tb := parseGFMTable(lines, tables[0].start, tables[0].end)
	if tb.rows[0][0] != "x | y" {
		t.Fatalf("escaped pipe not decoded: %q", tb.rows[0][0])
	}
	if tb.rows[0][1] != "" {
		t.Fatalf("expected empty cell, got %q", tb.rows[0][1])
	}
}

func TestTableAlignment(t *testing.T) {
	src := "| Left | Right | Center |\n|:---|---:|:---:|\n| a | b | c |"
	lines := strings.Split(src, "\n")
	tables := detectTablesInLines(lines)
	tb := parseGFMTable(lines, tables[0].start, tables[0].end)
	want := []string{"left", "right", "center"}
	for i, w := range want {
		if tb.aligns[i] != w {
			t.Fatalf("col %d align = %q, want %q", i, tb.aligns[i], w)
		}
	}
	rendered := renderCodeTable(tb)
	// Data row: col1 "a" flush left in a 4-wide cell, col2 "b" flush right,
	// col3 "c" centered.
	dataRow := "│ a    │     b │   c    │"
	if !strings.Contains(rendered, dataRow) {
		t.Fatalf("alignment not visible, want %q in:\n%s", dataRow, rendered)
	}
}

func TestTableInsideCodeBlockIgnored(t *testing.T) {
	src := "```python\nprint(\"+----+----+\")\nprint(\"| a  | b  |\")\nprint(\"+----+----+\")\n```"
	got := renderTablesIntoMarkdown(src, tableModeAuto, 60)
	if got != src {
		t.Fatalf("code block was modified:\n%s", got)
	}
}

func TestTableAutoFallsBackToCards(t *testing.T) {
	// Build a wide table (7 long columns) — must render as cards, not box.
	hdr := []string{"Параметр контроллера", "Тип шины", "Номер канала", "Скорость arbitration кбитс", "Скорость data кбитс", "Маска фильтров ID", "Замечания по конфигурации"}
	sep := make([]string, len(hdr))
	for i := range sep {
		sep[i] = "---"
	}
	row := []string{"FDCAN1", "High-Speed", "1", "500", "2000", "0x7FF", "основной канал"}
	join := func(xs []string) string { return "| " + strings.Join(xs, " | ") + " |" }
	src := join(hdr) + "\n" + join(sep) + "\n" + join(row)
	got := renderTablesIntoMarkdown(src, tableModeAuto, 60)
	if strings.Contains(got, "┌") {
		t.Fatalf("wide table should not render as box:\n%s", got)
	}
	if !strings.Contains(got, "▸ FDCAN1") || !strings.Contains(got, "Тип шины: High-Speed") {
		t.Fatalf("cards rendering missing:\n%s", got)
	}
}

func TestTableRaggedRowsPadded(t *testing.T) {
	src := "| Канал | Привод |\n|:---|:---|\n| can0 | FDCAN1 |\n| can1 |"
	got := renderTablesIntoMarkdown(src, tableModeCode, 60)
	if strings.Contains(got, "can1 |") {
		t.Fatalf("raw row leaked:\n%s", got)
	}
	if !strings.Contains(got, "can1") {
		t.Fatalf("row lost:\n%s", got)
	}
}

func TestTableOffKeepsSource(t *testing.T) {
	src := "| A |\n|---|\n| 1 |"
	got := renderTablesIntoMarkdown(src, tableModeOff, 60)
	if got != src {
		t.Fatalf("off mode modified the text:\n%s", got)
	}
}

func TestCJKEmojiWidth(t *testing.T) {
	tb := &gfmTable{headers: []string{"Статус", "Режим"}, aligns: []string{"left", "left"}, rows: [][]string{{"✅ готов", "CAN FD"}}}
	box := renderCodeTable(tb)
	var widths []int
	for _, ln := range strings.Split(box, "\n") {
		widths = append(widths, displayWidth(ln))
	}
	for _, w := range widths[1:] {
		if w != widths[0] {
			t.Fatalf("box rows have unequal display widths %v:\n%s", widths, box)
		}
	}
}

func TestNoFalsePositiveOnPipesInProse(t *testing.T) {
	src := "a | b\nsome text\n`x | y` remains"
	got := renderTablesIntoMarkdown(src, tableModeAuto, 60)
	if got != src {
		t.Fatalf("prose modified:\n%s", got)
	}
}
