package telegram

import (
	"regexp"
	"strings"

	"github.com/mattn/go-runewidth"
)

// GFM pipe-table support.
//
// Telegram Bot API has no table entity in either HTML or MarkdownV2 parse
// mode, so tables must be rendered into plain text. Two renderings exist:
//
//   - "code": an ASCII box-drawing table inside a monospace code block;
//   - "list": one card per row ("**header**" + "field: value" lines) used
//     when the table is too wide for a phone screen (Telegram has no
//     horizontal scroll inside <pre>).

const (
	tableModeAuto = "auto" // default: code when it fits, cards when too wide
	tableModeCode = "code"
	tableModeList = "list"
	tableModeOff  = "off" // leave table source untouched (legacy behavior)

	defaultTableMaxWidth = 60 // display columns of the rendered code table
	tableCellMaxWidth    = 30 // per-cell display width before truncation
)

// pipePlaceholder protects an escaped pipe \| inside a cell from being
// treated as a column separator during splitting.
// pipePlaceholder protects an escaped pipe inside a cell from being treated
// as a column separator during splitting.
const pipePlaceholder = "\x01"

var reTableSepCell = regexp.MustCompile(`^:?-{1,}:?$`)

// gfmTable is a parsed GFM pipe table.
type gfmTable struct {
	headers []string
	aligns  []string // "left" | "center" | "right"
	rows    [][]string
}

// isTableCellLine reports whether the line looks like a table row:
// it must contain a pipe outside of inline code backticks.
func isTableCellLine(line string) bool {
	t := strings.TrimSpace(line)
	if t == "" || !strings.Contains(strings.ReplaceAll(t, "\\|", ""), "|") {
		return false
	}
	// A line that is purely inline code (`a | b`) is not a table row.
	if strings.HasPrefix(t, "`") && strings.HasSuffix(t, "`") && strings.Count(t, "`") == 2 {
		return false
	}
	return true
}

// splitTableCells splits one table line into trimmed cells, honoring \| escapes
// and dropping the optional leading/trailing pipes.
func splitTableCells(line string) []string {
	t := strings.TrimSpace(line)
	t = strings.ReplaceAll(t, "\\|", pipePlaceholder)
	t = strings.TrimPrefix(t, "|")
	t = strings.TrimSuffix(t, "|")
	parts := strings.Split(t, "|")
	cells := make([]string, len(parts))
	for i, p := range parts {
		cells[i] = strings.TrimSpace(strings.ReplaceAll(p, pipePlaceholder, "|"))
	}
	return cells
}

// parseTableSeparator parses the alignment row. Returns nil when the line is
// not a valid separator (each cell must match :?---+:?).
func parseTableSeparator(line string) []string {
	if !isTableCellLine(line) {
		return nil
	}
	cells := splitTableCells(line)
	if len(cells) == 0 {
		return nil
	}
	aligns := make([]string, len(cells))
	for i, c := range cells {
		if c == "" || !reTableSepCell.MatchString(c) {
			return nil
		}
		left := strings.HasPrefix(c, ":")
		right := strings.HasSuffix(c, ":")
		switch {
		case left && right:
			aligns[i] = "center"
		case right:
			aligns[i] = "right"
		default:
			aligns[i] = "left"
		}
	}
	return aligns
}

// detectTablesInLines scans lines (with fenced-code awareness) and returns the
// table segments found. A table is a header line, a separator line and zero or
// more following cell lines.
func detectTablesInLines(lines []string) []struct{ start, end int } {
	var found []struct{ start, end int }
	inFence := false
	for i := 0; i+1 < len(lines); i++ {
		line := lines[i]
		if isFenceLine(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if !isTableCellLine(line) {
			continue
		}
		aligns := parseTableSeparator(lines[i+1])
		if aligns == nil {
			continue
		}
		end := i + 2
		for end < len(lines) && isTableCellLine(lines[end]) && !isFenceLine(lines[end]) {
			end++
		}
		found = append(found, struct{ start, end int }{i, end})
		i = end - 1
	}
	return found
}

func isFenceLine(line string) bool {
	t := strings.TrimSpace(line)
	return strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~")
}

// parseGFMTable parses the [start,end) line range (already detected) into a table.
func parseGFMTable(lines []string, start, end int) *gfmTable {
	t := &gfmTable{}
	t.headers = splitTableCells(lines[start])
	t.aligns = parseTableSeparator(lines[start+1])
	for i := start + 2; i < end; i++ {
		t.rows = append(t.rows, splitTableCells(lines[i]))
	}
	// Pad/trim rows to the header width.
	n := len(t.headers)
	for i := range t.rows {
		for len(t.rows[i]) < n {
			t.rows[i] = append(t.rows[i], "")
		}
		if len(t.rows[i]) > n {
			t.rows[i] = t.rows[i][:n]
		}
	}
	return t
}

// stripCellInline removes inline markdown markers for rendering inside a
// monospace code block (backticks, bold/italic markers, link syntax).
func stripCellInline(s string) string {
	s = reLink.ReplaceAllString(s, "$1")
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "__", "")
	s = strings.ReplaceAll(s, "~~", "")
	s = strings.ReplaceAll(s, "`", "")
	return strings.TrimSpace(s)
}

// cellDisplay prepares a cell for the ASCII renderer.
func cellDisplay(s string, maxW int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = stripCellInline(s)
	if runewidth.StringWidth(s) > maxW {
		s = strings.TrimSpace(runewidth.Truncate(s, maxW, "…"))
	}
	return s
}

// width returns the display width of the cell text.
func displayWidth(s string) int { return runewidth.StringWidth(s) }

// padCell aligns text to the given display width.
func padCell(s string, w int, align string) string {
	pad := w - displayWidth(s)
	if pad <= 0 {
		return s
	}
	switch align {
	case "right":
		return strings.Repeat(" ", pad) + s
	case "center":
		left := pad / 2
		return strings.Repeat(" ", left) + s + strings.Repeat(" ", pad-left)
	default:
		return s + strings.Repeat(" ", pad)
	}
}

// renderCodeTable renders the table as a Unicode box-drawing block.
func renderCodeTable(t *gfmTable) string {
	n := len(t.headers)
	cols := make([]string, n)
	for i, h := range t.headers {
		cols[i] = cellDisplay(h, tableCellMaxWidth)
	}
	rows := make([][]string, len(t.rows))
	for r, row := range t.rows {
		rows[r] = make([]string, n)
		for i := 0; i < n; i++ {
			rows[r][i] = cellDisplay(row[i], tableCellMaxWidth)
		}
	}
	widths := make([]int, n)
	for i := 0; i < n; i++ {
		widths[i] = displayWidth(cols[i])
		for _, row := range rows {
			if w := displayWidth(row[i]); w > widths[i] {
				widths[i] = w
			}
		}
	}
	var b strings.Builder
	writeRule := func(l, m, r string) {
		b.WriteString(l)
		for i := 0; i < n; i++ {
			b.WriteString(strings.Repeat("─", widths[i]+2))
			if i < n-1 {
				b.WriteString(m)
			}
		}
		b.WriteString(r)
		b.WriteByte('\n')
	}
	writeRow := func(cells []string) {
		b.WriteString("│")
		for i := 0; i < n; i++ {
			align := "left"
			if i < len(t.aligns) {
				align = t.aligns[i]
			}
			b.WriteString(" " + padCell(cells[i], widths[i], align) + " ")
			b.WriteString("│")
		}
		b.WriteByte('\n')
	}
	writeRule("┌", "┬", "┐")
	writeRow(cols)
	writeRule("├", "┼", "┤")
	for _, row := range rows {
		writeRow(row)
	}
	writeRule("└", "┴", "┘")
	return strings.TrimSuffix(b.String(), "\n")
}

// codeTableWidth is the rendered display width of the box table.
func codeTableWidth(t *gfmTable) int {
	w := 1 // left border
	for i, h := range t.headers {
		cw := displayWidth(cellDisplay(h, tableCellMaxWidth))
		for _, row := range t.rows {
			if i < len(row) {
				if rw := displayWidth(cellDisplay(row[i], tableCellMaxWidth)); rw > cw {
					cw = rw
				}
			}
		}
		w += cw + 2 + 1
	}
	return w
}

// renderTable renders the parsed table into a markdown snippet and a flag
// telling whether the snippet must be wrapped in a fenced code block.
func renderTable(t *gfmTable, mode string, maxW int) (string, bool) {
	if maxW <= 0 {
		maxW = defaultTableMaxWidth
	}
	useCode := false
	switch mode {
	case tableModeCode:
		useCode = true
	case tableModeList:
		useCode = false
	default: // auto
		useCode = codeTableWidth(t) <= maxW && len(t.headers) > 0
	}
	if useCode {
		return renderCodeTable(t), true
	}
	return cardsAsMarkdown(t), false
}

// renderTablesIntoMarkdown rewrites every GFM pipe table in the source into
// a markdown form the Telegram HTML/MarkdownV2 converters render correctly:
// either a fenced box-drawing block (code) or bold "field: value" cards
// (list). Tables inside fenced code blocks are left untouched. In mode
// "off" the text is returned unchanged.
func renderTablesIntoMarkdown(text, mode string, maxW int) string {
	if mode == tableModeOff {
		return text
	}
	if strings.TrimSpace(text) == "" {
		return text
	}
	lines := strings.Split(text, "\n")
	tables := detectTablesInLines(lines)
	if len(tables) == 0 {
		return text
	}
	var out []string
	pos := 0
	for _, tb := range tables {
		out = append(out, lines[pos:tb.start]...)
		t := parseGFMTable(lines, tb.start, tb.end)
		rendered, codeBlock := renderTable(t, mode, maxW)
		if codeBlock {
			out = append(out, "```", rendered, "```")
		} else {
			out = append(out, strings.Split(cardsAsMarkdown(t), "\n")...)
		}
		pos = tb.end
	}
	out = append(out, lines[pos:]...)
	return strings.Join(out, "\n")
}

// cardsAsMarkdown renders rows as bold card titles with bullet "field: value"
// lines — ordinary markdown that both Telegram converters handle natively.
func cardsAsMarkdown(t *gfmTable) string {
	var b strings.Builder
	for _, row := range t.rows {
		title := stripCellInline(row[0])
		if title == "" {
			title = stripCellInline(t.headers[0])
		}
		if title == "" {
			title = "—"
		}
		b.WriteString("**▸ " + title + "**\n")
		for i := 1; i < len(t.headers); i++ {
			field := stripCellInline(t.headers[i])
			val := stripCellInline(row[i])
			b.WriteString("- " + field + ": " + val + "\n")
		}
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
