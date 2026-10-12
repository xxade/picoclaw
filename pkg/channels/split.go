package channels

import (
	"strings"
)

// gfmTableBlock locates a GFM pipe table (header + separator + rows) that sits
// outside fenced code blocks. All offsets are rune indices.
type gfmTableBlock struct {
	lineStart int    // first rune of the header line
	headerEnd int    // first rune after the header line's newline
	sepEnd    int    // first rune after the separator line's newline
	end       int    // first rune after the last table row
	header    string // header line without its newline
	sep       string // separator line without its newline
}

// findGFMTableBlocks scans runes[start:] for GFM tables outside code fences.
// Only leading-pipe rows are treated as table rows, which is what markdown
// emitting LLMs produce and keeps the scan conservative.
func findGFMTableBlocks(runes []rune, start, end int) []gfmTableBlock {
	type line struct {
		text       string
		start, end int
	}
	var lines []line
	pos := start
	for pos < end {
		idx := pos
		for idx < end && runes[idx] != '\n' {
			idx++
		}
		lineEnd := idx + 1
		if lineEnd > end {
			lineEnd = end
		}
		lines = append(lines, line{string(runes[pos:lineEnd]), pos, lineEnd})
		pos = lineEnd
	}

	var blocks []gfmTableBlock
	inFence := false
	for i := 0; i+1 < len(lines); i++ {
		t := strings.TrimSpace(lines[i].text)
		if strings.HasPrefix(t, "```") || strings.HasPrefix(t, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if isTableRowLine(t) && isTableSepLine(strings.TrimSpace(lines[i+1].text)) {
			j := i + 2
			for j < len(lines) && isTableRowLine(strings.TrimSpace(lines[j].text)) {
				j++
			}
			blocks = append(blocks, gfmTableBlock{
				lineStart: lines[i].start,
				headerEnd: lines[i].end,
				sepEnd:    lines[i+1].end,
				end:       lines[j-1].end,
				header:    strings.TrimRight(lines[i].text, "\r\n"),
				sep:       strings.TrimRight(lines[i+1].text, "\r\n"),
			})
			i = j - 1
		}
	}
	return blocks
}

// tableCutAt reports the GFM table that runes[start:totalLen] contains with a
// boundary at msgEnd cutting through it (between the header and the last row).
// Returns nil when msgEnd does not cut a table.
func tableCutAt(runes []rune, start, totalLen, msgEnd int) *gfmTableBlock {
	for _, b := range findGFMTableBlocks(runes, start, totalLen) {
		if msgEnd > b.lineStart && msgEnd < b.end {
			return &b
		}
	}
	return nil
}

// lastNewlineAtMost returns the index just after the last newline in
// runes[from:to], or from when there is none.
func lastNewlineAtMost(runes []rune, from, to int) int {
	for i := to - 1; i >= from; i-- {
		if runes[i] == '\n' {
			return i + 1
		}
	}
	return from
}

func isTableRowLine(trimmed string) bool {
	return strings.HasPrefix(trimmed, "|") && strings.Count(trimmed, "|") >= 2
}

func isTableSepLine(trimmed string) bool {
	if !strings.HasPrefix(trimmed, "|") {
		return false
	}
	cells := strings.Split(strings.Trim(trimmed, "|"), "|")
	if len(cells) < 2 {
		return false
	}
	for _, c := range cells {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		for _, r := range c {
			if r != '-' && r != ':' {
				return false
			}
		}
	}
	return true
}

// SplitMessage splits long messages into chunks, preserving code block integrity.
// The maxLen parameter is measured in runes (Unicode characters), not bytes.
// The function reserves a buffer (10% of maxLen, min 50) to leave room for closing code blocks,
// but may extend to maxLen when needed.
// Call SplitMessage with the full text content and the maximum allowed length of a single message;
// it returns a slice of message chunks that each respect maxLen and avoid splitting fenced code blocks.
func SplitMessage(content string, maxLen int) []string {
	if maxLen <= 0 {
		if content == "" {
			return nil
		}
		return []string{content}
	}

	runes := []rune(content)
	totalLen := len(runes)
	var messages []string

	// Dynamic buffer: 10% of maxLen, but at least 50 chars if possible
	codeBlockBuffer := max(maxLen/10, 50)
	if codeBlockBuffer > maxLen/2 {
		codeBlockBuffer = maxLen / 2
	}

	start := 0
	for start < totalLen {
		remaining := totalLen - start
		if remaining <= maxLen {
			messages = append(messages, string(runes[start:totalLen]))
			break
		}

		// Effective split point: maxLen minus buffer, to leave room for code blocks
		effectiveLimit := max(maxLen-codeBlockBuffer, maxLen/2)

		end := start + effectiveLimit

		// Find natural split point within the effective limit
		msgEnd := findLastNewlineInRange(runes, start, end, 200)
		if msgEnd <= start {
			msgEnd = findLastSpaceInRange(runes, start, end, 100)
		}
		if msgEnd <= start {
			msgEnd = end
		}

		// Check if this would end with an incomplete code block
		unclosedIdx := findLastUnclosedCodeBlockInRange(runes, start, msgEnd)

		if unclosedIdx >= 0 {
			// Message would end with incomplete code block
			// Try to extend up to maxLen to include the closing ```
			if totalLen > msgEnd {
				closingIdx := findNextClosingCodeBlockInRange(runes, msgEnd, totalLen)
				if closingIdx > 0 && closingIdx-start <= maxLen {
					// Extend to include the closing ```
					msgEnd = closingIdx
				} else {
					// Code block is too long to fit in one chunk or missing closing fence.
					// Try to split inside by injecting closing and reopening fences.
					headerEnd := findNewlineFrom(runes, unclosedIdx)
					var header string
					if headerEnd == -1 {
						header = strings.TrimSpace(string(runes[unclosedIdx : unclosedIdx+3]))
					} else {
						header = strings.TrimSpace(string(runes[unclosedIdx:headerEnd]))
					}
					headerEndIdx := unclosedIdx + len([]rune(header))
					if headerEnd != -1 {
						headerEndIdx = headerEnd
					}

					// If we have a reasonable amount of content after the header, split inside
					if msgEnd > headerEndIdx+20 {
						// Find a better split point closer to maxLen
						innerLimit := min(
							// Leave room for "\n```"
							start+maxLen-5, totalLen)
						betterEnd := findLastNewlineInRange(runes, start, innerLimit, 200)
						if betterEnd > headerEndIdx {
							msgEnd = betterEnd
						} else {
							msgEnd = innerLimit
						}
						chunk := strings.TrimRight(string(runes[start:msgEnd]), " \t\n\r") + "\n```"
						messages = append(messages, chunk)
						remaining := strings.TrimSpace(header + "\n" + string(runes[msgEnd:totalLen]))
						// Replace the tail of runes with the reconstructed remaining
						runes = []rune(remaining)
						totalLen = len(runes)
						start = 0
						continue
					}

					// Otherwise, try to split before the code block starts
					newEnd := findLastNewlineInRange(runes, start, unclosedIdx, 200)
					if newEnd <= start {
						newEnd = findLastSpaceInRange(runes, start, unclosedIdx, 100)
					}
					if newEnd > start {
						msgEnd = newEnd
					} else {
						// If we can't split before, we MUST split inside (last resort)
						if unclosedIdx-start > 20 {
							msgEnd = unclosedIdx
						} else {
							splitAt := min(start+maxLen-5, totalLen)
							chunk := strings.TrimRight(string(runes[start:splitAt]), " \t\n\r") + "\n```"
							messages = append(messages, chunk)
							remaining := strings.TrimSpace(header + "\n" + string(runes[splitAt:totalLen]))
							runes = []rune(remaining)
							totalLen = len(runes)
							start = 0
							continue
						}
					}
				}
			}
		}

		if msgEnd <= start {
			msgEnd = start + effectiveLimit
		}

		// Never cut a GFM table mid-row: a fragment without its header line
		// renders as loose pipe text instead of a table. Prefer moving the
		// boundary before the whole table; if the table alone cannot fit one
		// chunk, split at a row boundary and re-open the header in the next
		// chunk so every fragment stays a valid table.
		if cut := tableCutAt(runes, start, totalLen, msgEnd); cut != nil {
			if cut.lineStart > start {
				msgEnd = cut.lineStart
			} else if rowEnd := lastNewlineAtMost(runes, cut.sepEnd, msgEnd); rowEnd > cut.sepEnd {
				messages = append(messages, strings.TrimRight(string(runes[start:rowEnd]), " 	\r\n"))
				runes = []rune(cut.header + "\n" + cut.sep + "\n" + string(runes[rowEnd:totalLen]))
				totalLen = len(runes)
				start = 0
				continue
			}
		}

		messages = append(messages, string(runes[start:msgEnd]))
		// Advance start, skipping leading whitespace of next chunk
		start = msgEnd
		for start < totalLen && (runes[start] == ' ' || runes[start] == '\t' || runes[start] == '\n' || runes[start] == '\r') {
			start++
		}
	}

	return messages
}

// findLastUnclosedCodeBlockInRange finds the last opening ``` that doesn't have a closing ```
// within runes[start:end]. Returns the absolute rune index or -1.
func findLastUnclosedCodeBlockInRange(runes []rune, start, end int) int {
	inCodeBlock := false
	lastOpenIdx := -1

	for i := start; i < end; i++ {
		if i+2 < end && runes[i] == '`' && runes[i+1] == '`' && runes[i+2] == '`' {
			if !inCodeBlock {
				lastOpenIdx = i
			}
			inCodeBlock = !inCodeBlock
			i += 2
		}
	}

	if inCodeBlock {
		return lastOpenIdx
	}
	return -1
}

// findNextClosingCodeBlockInRange finds the next closing ``` starting from startIdx
// within runes[startIdx:end]. Returns the absolute index after the closing ``` or -1.
func findNextClosingCodeBlockInRange(runes []rune, startIdx, end int) int {
	for i := startIdx; i < end; i++ {
		if i+2 < end && runes[i] == '`' && runes[i+1] == '`' && runes[i+2] == '`' {
			return i + 3
		}
	}
	return -1
}

// findNewlineFrom finds the first newline character starting from the given index.
// Returns the absolute index or -1 if not found.
func findNewlineFrom(runes []rune, from int) int {
	for i := from; i < len(runes); i++ {
		if runes[i] == '\n' {
			return i
		}
	}
	return -1
}

// findLastNewlineInRange finds the last newline within the last searchWindow runes
// of the range runes[start:end]. Returns the absolute index or start-1 (indicating not found).
func findLastNewlineInRange(runes []rune, start, end, searchWindow int) int {
	searchStart := max(end-searchWindow, start)
	for i := end - 1; i >= searchStart; i-- {
		if runes[i] == '\n' {
			return i
		}
	}
	return start - 1
}

// findLastSpaceInRange finds the last space/tab within the last searchWindow runes
// of the range runes[start:end]. Returns the absolute index or start-1 (indicating not found).
func findLastSpaceInRange(runes []rune, start, end, searchWindow int) int {
	searchStart := max(end-searchWindow, start)
	for i := end - 1; i >= searchStart; i-- {
		if runes[i] == ' ' || runes[i] == '\t' {
			return i
		}
	}
	return start - 1
}
