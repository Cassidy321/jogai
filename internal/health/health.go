package health

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Cassidy321/jogai/internal/devday"
	"github.com/Cassidy321/jogai/internal/lastrun"
	"github.com/Cassidy321/jogai/internal/update"
)

const recentDays = 14

type Inputs struct {
	Now           time.Time
	LastIngest    time.Time
	FormatWarning string
}

// jogai runs unattended and nobody reads its logs: whatever needs the user's
// attention is said by Claude at the start of a session.
func Warnings(in Inputs) []string {
	var out []string
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
		day, err := time.ParseInLocation(devday.LabelFormat, label, in.Now.Location())
		if err != nil || in.Now.Sub(day) > recentDays*24*time.Hour {
			continue
		}
		d := days[label]
		switch d.Status {
		case lastrun.StatusError:
			first, _, _ := strings.Cut(d.Error, "\n")
			out = append(out, fmt.Sprintf("the recap of %s failed and will be retried on the next run: %s", label, first))
		case lastrun.StatusRefused:
			out = append(out, fmt.Sprintf("the model refused to summarize %s; regenerate it with `jogai run --day %s`", label, label))
		case lastrun.StatusOK, lastrun.StatusPartial, lastrun.StatusEmpty:
		}
	}
	if !in.LastIngest.IsZero() && in.Now.Sub(in.LastIngest) > 72*time.Hour {
		out = append(out, fmt.Sprintf("the session archive has not been updated since %s — is the schedule still running? (`jogai status`)", in.LastIngest.Format("2006-01-02")))
	}
	if in.FormatWarning != "" {
		out = append(out, in.FormatWarning)
	}
	if e := update.LastError(); e != "" {
		out = append(out, "jogai could not update itself: "+e)
	}
	return out
}
