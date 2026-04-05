package filter

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Cassidy321/jogai/internal/parser"
)

const (
	MaxAssistantChars = 2000
	marker            = "\n[...]\n"
	markerRuneLen     = 7
)

func Reduce(sessions []parser.Session) []parser.Session {
	result := make([]parser.Session, 0, len(sessions))
	for _, s := range sessions {
		filtered := reduceSession(s)
		if len(filtered.Messages) > 0 {
			result = append(result, filtered)
		}
	}
	return result
}

func reduceSession(s parser.Session) parser.Session {
	messages := make([]parser.Message, 0, len(s.Messages))
	for _, m := range s.Messages {
		if m.Role == "assistant" {
			m.Content = collapseCodeBlocks(m.Content)
			m.Content = truncateRunes(m.Content, MaxAssistantChars)
		}
		messages = append(messages, m)
	}
	s.Messages = messages
	return s
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

func truncateRunes(s string, maxRunes int) string {
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
