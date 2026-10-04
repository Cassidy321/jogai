package filter

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/Cassidy321/jogai/internal/parser"
)

const (
	MaxAssistantChars = 2000
	MaxUserChars      = 4000
	// Heavy days (600k+ characters) otherwise hit the summarizer timeout.
	MaxPromptChars = 300_000
	marker         = "\n[...]\n"
	markerRuneLen  = 7
)

func Reduce(sessions []parser.Session) []parser.Session {
	return reduce(sessions, MaxPromptChars)
}

func reduce(sessions []parser.Session, budget int) []parser.Session {
	result := make([]parser.Session, 0, len(sessions))
	for _, s := range sessions {
		filtered := reduceSession(s)
		if len(filtered.Messages) > 0 {
			result = append(result, filtered)
		}
	}
	fitBudget(result, budget)
	return result
}

func reduceSession(s parser.Session) parser.Session {
	messages := make([]parser.Message, 0, len(s.Messages))
	for _, m := range s.Messages {
		if m.Role == "assistant" {
			m.Content = Truncate(collapseCodeBlocks(m.Content), MaxAssistantChars)
		} else {
			m.Content = Truncate(m.Content, MaxUserChars)
		}
		messages = append(messages, m)
	}
	s.Messages = messages
	return s
}

// Caps the longest messages first so short ones, most of what the user typed,
// are never cut to make room for a few huge pastes or answers.
func fitBudget(sessions []parser.Session, budget int) {
	var lengths []int
	total := 0
	for _, s := range sessions {
		for _, m := range s.Messages {
			n := utf8.RuneCountInString(m.Content)
			lengths = append(lengths, n)
			total += n
		}
	}
	if total <= budget {
		return
	}
	limit := capFor(lengths, budget)
	for i := range sessions {
		for j := range sessions[i].Messages {
			sessions[i].Messages[j].Content = Truncate(sessions[i].Messages[j].Content, limit)
		}
	}
}

func capFor(lengths []int, budget int) int {
	lo, hi := markerRuneLen+1, slices.Max(lengths)
	for lo < hi {
		mid := (lo + hi + 1) / 2
		if cappedSum(lengths, mid) <= budget {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	return lo
}

func cappedSum(lengths []int, limit int) int {
	sum := 0
	for _, n := range lengths {
		sum += min(n, limit)
	}
	return sum
}

func collapseCodeBlocks(s string) string {
	var b strings.Builder

	for {
		fenceStart, fenceLen := findOpeningFence(s)
		if fenceStart == -1 {
			b.WriteString(s)
			break
		}

		b.WriteString(s[:fenceStart])

		rest := s[fenceStart+fenceLen:]

		lang := ""
		if nl := strings.IndexByte(rest, '\n'); nl != -1 {
			lang = strings.TrimSpace(rest[:nl])
			rest = rest[nl+1:]
		}

		closingFence := strings.Repeat("`", fenceLen)
		end := strings.Index(rest, closingFence)
		if end == -1 {
			b.WriteString(s[fenceStart:])
			break
		}

		codeContent := strings.TrimRight(rest[:end], "\n")
		lineCount := 0
		if codeContent != "" {
			lineCount = strings.Count(codeContent, "\n") + 1
		}

		if lang != "" {
			fmt.Fprintf(&b, "[code block: %s, %d lines]\n", lang, lineCount)
		} else {
			fmt.Fprintf(&b, "[code block: %d lines]\n", lineCount)
		}

		after := rest[end+fenceLen:]
		if len(after) > 0 && after[0] == '\n' {
			after = after[1:]
		}
		s = after
	}

	return b.String()
}

func findOpeningFence(s string) (pos int, length int) {
	offset := 0
	for {
		i := strings.Index(s[offset:], "```")
		if i == -1 {
			return -1, 0
		}
		i += offset
		if i == 0 || s[i-1] == '\n' {
			n := 3
			for i+n < len(s) && s[i+n] == '`' {
				n++
			}
			return i, n
		}
		offset = i + 3
	}
}

func Truncate(s string, maxRunes int) string {
	if maxRunes <= markerRuneLen {
		return s[:0]
	}

	runeCount := utf8.RuneCountInString(s)
	if runeCount <= maxRunes {
		return s
	}

	budget := maxRunes - markerRuneLen
	half := budget / 2

	headEnd := 0
	for i := 0; i < half; i++ {
		_, size := utf8.DecodeRuneInString(s[headEnd:])
		headEnd += size
	}

	tailStart := len(s)
	for i := 0; i < half; i++ {
		_, size := utf8.DecodeLastRuneInString(s[:tailStart])
		tailStart -= size
	}

	return s[:headEnd] + marker + s[tailStart:]
}
