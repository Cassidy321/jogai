package secrets

import (
	"regexp"
	"strings"
)

const placeholder = "[secret]"

// When a pattern has a capture group, only that group is the secret: the key
// name or URL around it stays readable in recaps and search results.
var patterns = []*regexp.Regexp{
	regexp.MustCompile(`eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`),
	regexp.MustCompile(`\b(?:sk-(?:ant-)?[A-Za-z0-9_-]{20,}|ghp_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{30,}|AKIA[0-9A-Z]{16}|xox[abprs]-[A-Za-z0-9-]{10,})`),
	regexp.MustCompile(`(?i)\bbearer\s+([A-Za-z0-9._~+/=-]{20,})`),
	// "[" is excluded so a value an earlier pattern already masked is not counted twice.
	regexp.MustCompile(`(?i)(?:password|passwd|pwd|secret|api[_-]?key|access[_-]?token|auth[_-]?token|client[_-]?secret)["']?\s*[:=]\s*["']?([^\s"',;&\[]{6,})`),
	regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.-]*://[^\s:/@]+:([^\s@/]+)@`),
}

func Mask(text string) (string, int) {
	total := 0
	for _, re := range patterns {
		var n int
		text, n = replace(re, text)
		total += n
	}
	return text, total
}

func replace(re *regexp.Regexp, text string) (string, int) {
	matches := re.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return text, 0
	}
	var b strings.Builder
	last := 0
	for _, m := range matches {
		start, end := m[0], m[1]
		if len(m) >= 4 && m[2] >= 0 {
			start, end = m[2], m[3]
		}
		b.WriteString(text[last:start])
		b.WriteString(placeholder)
		last = end
	}
	b.WriteString(text[last:])
	return b.String(), len(matches)
}
