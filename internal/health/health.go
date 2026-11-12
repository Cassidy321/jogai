package health

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Cassidy321/jogai/internal/claudetoken"
	"github.com/Cassidy321/jogai/internal/devday"
	"github.com/Cassidy321/jogai/internal/lastrun"
	"github.com/Cassidy321/jogai/internal/summary"
	"github.com/Cassidy321/jogai/internal/update"
)

const recentDays = 14

type Inputs struct {
	Now           time.Time
	LastIngest    time.Time
	FormatWarning string
	Summarizer    string
}

// jogai runs unattended and nobody reads its logs: whatever needs the user's
// attention is said by Claude at the start of a session.
func Warnings(in Inputs) []string {
	return append(dayWarnings(in.Now, in.Summarizer), System(in)...)
}

func Cause(kind, summarizer string) string {
	switch {
	case kind == lastrun.KindConfig:
		return "Codex refuses the model set in ~/.codex/config.toml"
	case summarizer == summary.NameCodex:
		return "jogai cannot log in to Codex"
	default:
		return "jogai cannot log in to Claude"
	}
}

func Fix(kind, summarizer string) string {
	switch {
	case kind == lastrun.KindConfig:
		return "open Codex and pick another model with /model"
	case summarizer == summary.NameCodex:
		return "run `codex login` in a terminal"
	default:
		return "run `jogai login` in a terminal"
	}
}

// System leaves out the dev days, which `jogai status` lists in its own terms.
func System(in Inputs) []string {
	var out []string
	if !in.LastIngest.IsZero() && in.Now.Sub(in.LastIngest) > 72*time.Hour {
		out = append(out, fmt.Sprintf("the session archive has not been updated since %s — is the schedule still running? (`jogai status`)", in.LastIngest.Format("2006-01-02")))
	}
	if in.FormatWarning != "" {
		out = append(out, in.FormatWarning)
	}
	if e := update.LastError(); e != "" {
		out = append(out, "jogai could not update itself: "+e)
	}
	if st, ok := claudetoken.LoadState(); ok && in.Summarizer != summary.NameCodex && st.ExpiresSoon(in.Now) {
		out = append(out, tokenWarning(st, in.Now))
	}
	return out
}

// An expired token fails like any other login failure: the date kept at
// `jogai login` is the only way to warn before it happens.
func tokenWarning(st claudetoken.State, now time.Time) string {
	date := st.ExpiresAt().Format("2006-01-02")
	if st.Expired(now) {
		return fmt.Sprintf("jogai's Claude token expired on %s — run `jogai login` in a terminal", date)
	}
	left := "in less than a day"
	if days := int(st.ExpiresAt().Sub(now).Hours() / 24); days == 1 {
		left = "in 1 day"
	} else if days > 1 {
		left = fmt.Sprintf("in %d days", days)
	}
	return fmt.Sprintf("jogai's Claude token expires on %s (%s) — run `jogai login` in a terminal to renew it", date, left)
}

// A day stopped by the login or the setup fails again on every run: it gets one
// alert naming what to do, whatever the number of days behind it.
func dayWarnings(now time.Time, summarizer string) []string {
	var out []string
	stoppedAt := map[string]string{}
	days, err := lastrun.LoadDays()
	if err != nil {
		out = append(out, "jogai's run history is unreadable: "+err.Error())
	}
	labels := make([]string, 0, len(days))
	for label := range days {
		labels = append(labels, label)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(labels)))
	for _, label := range labels {
		day, err := time.ParseInLocation(devday.LabelFormat, label, now.Location())
		if err != nil || now.Sub(day) > recentDays*24*time.Hour {
			continue
		}
		d := days[label]
		switch d.Status {
		case lastrun.StatusError:
			if d.Kind == lastrun.KindAuth || d.Kind == lastrun.KindConfig {
				stoppedAt[d.Kind] = label
				continue
			}
			out = append(out, fmt.Sprintf("the recap of %s failed and will be retried on the next run: %s", label, firstError(d.Error)))
		case lastrun.StatusRefused:
			out = append(out, fmt.Sprintf("the model refused to summarize %s; regenerate it with `jogai run --day %s`", label, label))
		case lastrun.StatusOK, lastrun.StatusPartial, lastrun.StatusEmpty:
		}
	}
	var stopped []string
	for _, kind := range []string{lastrun.KindAuth, lastrun.KindConfig} {
		if label, ok := stoppedAt[kind]; ok {
			stopped = append(stopped, fmt.Sprintf("%s: recaps are stopped at the dev day %s — %s", Cause(kind, summarizer), label, Fix(kind, summarizer)))
		}
	}
	return append(stopped, out...)
}

// The lines of an error are not one per project (a CLI failure can span
// several), so the rest is pointed to, not counted.
func firstError(failure string) string {
	var lines []string
	for line := range strings.SplitSeq(failure, "\n") {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	switch len(lines) {
	case 0:
		return ""
	case 1:
		return lines[0]
	}
	return lines[0] + " (more in `jogai status`)"
}
