package secrets

import (
	"regexp"
	"strings"
)

const placeholder = "[secret]"

// When a pattern has a capture group, only that group is the secret: the key
// name or URL around it stays readable in recaps and search results.
var patterns = []*regexp.Regexp{
	// The body must look like a key: a message that only names the marker keeps its text.
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----(?:\\n|\s)+(?:[A-Za-z0-9+/=]{16,}(?:\\n|\s)*)+(?:-----END [A-Z ]*PRIVATE KEY-----)?`),
	regexp.MustCompile(`eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}`),
	// A Claude token pasted from `claude setup-token` is often wrapped by the terminal: its second line goes with it.
	regexp.MustCompile(`\b(?:sk-ant-[A-Za-z0-9_-]{20,}(?:\r?\n[A-Za-z0-9_-]{20,})?|sk-[A-Za-z0-9_-]{20,}|[sr]k_(?:live|test)_[A-Za-z0-9]{10,}|whsec_[A-Za-z0-9]{20,}|gh[opsur]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{30,}|AKIA[0-9A-Z]{16}|AIza[0-9A-Za-z_-]{35}|xox[abprs]-[A-Za-z0-9-]{10,})`),
	regexp.MustCompile(`(?i)\bbearer\s+([A-Za-z0-9._~+/=-]{20,})`),
	regexp.MustCompile(`(?i)\bauthorization["']?\s*[:=]\s*["']?basic\s+([A-Za-z0-9+/=]{8,})`),
	// "[" is excluded so a value an earlier pattern already masked is not counted twice.
	regexp.MustCompile(`(?i)(?:password|passwd|pwd|secret[_-]?access[_-]?key|secret[_-]?key|secret|api[_-]?key|access[_-]?token|auth[_-]?token|client[_-]?secret)["']?\s*[:=]\s*["']?([^\s"',;&\[]{6,})`),
	// Upper case only, and never a plain number: `token := lexer.Next()` and
	// MAX_TOKENS=4096 are everyday code, GITHUB_TOKEN=… is a secret.
	regexp.MustCompile(`\b[A-Z0-9_]*TOKEN["']?\s*[:=]\s*["']?(\d*[^\s"',;&\[\d][^\s"',;&\[]{5,})`),
	regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9+.-]*://[^\s:/@]+:([^\s@/]+)@`),
	regexp.MustCompile(`hooks\.slack\.com/services/([A-Za-z0-9/]{20,})`),
	regexp.MustCompile(`\b(?:mysql|mysqldump|mariadb)\b[^\n]*\s-p([^\s\[]{4,})`),
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
