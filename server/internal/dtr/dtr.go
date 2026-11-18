// Package dtr turns raw attendance logs into a Daily Time Record sheet.
// Official hours are settings, not constants: each agency sets its own.
package dtr

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Window is a span of minutes from midnight.
type Window struct{ Start, End int }

// Hours are the rules in force for one employee on one date.
type Hours struct {
	AM, PM      Window
	Grace       int   // minutes after the start of a period still counted as on time
	Flexi       bool  // any arrival inside the day counts; only total time matters
	Required    int   // minutes owed per day on flexitime
	LatestIn    int   // flexitime: arriving after this is late
	WorkDays    uint8 // bit 0 is Monday
	TwoLog      bool  // only arrival and departure are logged
	Overnight   bool  // the shift ends on the next calendar day
	SuspendFrom int   // minutes from midnight when work was suspended; -1 if not
}

const weekdaysOnly = 0b0011111

func (h Hours) Total() int {
	if h.Flexi {
		return h.Required
	}
	return (h.AM.End - h.AM.Start) + (h.PM.End - h.PM.Start)
}

// Slots lists the logs this schedule expects each day, in order.
func (h Hours) Slots() []string {
	if h.TwoLog {
		return []string{"am_in", "pm_out"}
	}
	return []string{"am_in", "am_out", "pm_in", "pm_out"}
}

func (h Hours) IsWorkDay(d time.Weekday) bool {
	return h.WorkDays&(1<<((int(d)+6)%7)) != 0
}

// LateAt says whether an arrival at this minute of the day counts as late.
func (h Hours) LateAt(minute int) bool {
	if h.Flexi {
		return h.LatestIn > 0 && minute > h.LatestIn
	}
	return minute > h.AM.Start+h.Grace
}

// ParseHours reads "08:00-12:00,13:00-17:00".
func ParseHours(s string) (Hours, error) {
	parts := strings.Split(s, ",")
	if len(parts) != 2 {
		return Hours{}, fmt.Errorf("official hours must look like 08:00-12:00,13:00-17:00")
	}
	var w [2]Window
	for i, p := range parts {
		ends := strings.Split(strings.TrimSpace(p), "-")
		if len(ends) != 2 {
			return Hours{}, fmt.Errorf("bad window %q", p)
		}
		a, err1 := minutes(ends[0])
		b, err2 := minutes(ends[1])
		if err1 != nil || err2 != nil || b <= a {
			return Hours{}, fmt.Errorf("bad window %q", p)
		}
		w[i] = Window{a, b}
	}
	if w[1].Start < w[0].End {
		return Hours{}, fmt.Errorf("afternoon window must start after the morning window ends")
	}
	return Hours{AM: w[0], PM: w[1], WorkDays: weekdaysOnly, SuspendFrom: -1,
		Required: (w[0].End - w[0].Start) + (w[1].End - w[1].Start)}, nil
}

// ParseMinutes reads "HH:MM" as minutes from midnight.
func ParseMinutes(hhmm string) (int, error) { return minutes(hhmm) }

func minutes(hhmm string) (int, error) {
	h, m, ok := strings.Cut(strings.TrimSpace(hhmm), ":")
	if !ok {
		return 0, fmt.Errorf("bad time")
	}
	hh, err1 := strconv.Atoi(h)
	mm, err2 := strconv.Atoi(m)
	if err1 != nil || err2 != nil || hh < 0 || hh > 23 || mm < 0 || mm > 59 {
		return 0, fmt.Errorf("bad time")
	}
	return hh*60 + mm, nil
}

// Log is one recorded slot, already in local (Asia/Manila) time.
type Log struct {
	Day      int
	Slot     string
	At       time.Time
	Adjusted bool // came from an approved correction, not the device
}

type Day struct {
	Day          int    `json:"day"`
	Weekday      string `json:"weekday"`
	AMIn         string `json:"am_in"`
	AMOut        string `json:"am_out"`
	PMIn         string `json:"pm_in"`
	PMOut        string `json:"pm_out"`
	UndertimeMin int    `json:"undertime_min"`
	Incomplete   bool   `json:"incomplete"`
	Adjusted     bool   `json:"adjusted"`
	Late         bool   `json:"late"`
	Rest         bool   `json:"rest"` // not a work day on this schedule
	Mark         string `json:"mark"` // e.g. LEAVE; the day is excused
}

type Sheet struct {
	Month             string `json:"month"` // YYYY-MM
	Days              []Day  `json:"days"`
	TotalUndertimeMin int    `json:"total_undertime_min"`
}

// Compute builds the sheet. hoursFor gives the official hours in force on a date,
// so a schedule change never rewrites earlier days. Undertime counts late arrival, early departure and
// unrecorded time inside official hours, for days that have at least one log.
// Days with an approved leave, official business or travel order carry no
// undertime. A day missing a pair is flagged Incomplete so a person reviews it, because
// the figure for that day is then an upper bound.
func Compute(year int, month time.Month, logs []Log, marks map[int]string, hoursFor func(time.Time) Hours, loc *time.Location) Sheet {
	byDay := map[int]map[string]time.Time{}
	adjusted := map[int]bool{}
	for _, l := range logs {
		if byDay[l.Day] == nil {
			byDay[l.Day] = map[string]time.Time{}
		}
		byDay[l.Day][l.Slot] = l.At
		if l.Adjusted {
			adjusted[l.Day] = true
		}
	}

	first := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	n := first.AddDate(0, 1, -1).Day()
	sheet := Sheet{Month: first.Format("2006-01")}

	for d := 1; d <= n; d++ {
		date := time.Date(year, month, d, 0, 0, 0, 0, loc)
		h := hoursFor(date)
		row := Day{Day: d, Weekday: date.Weekday().String(), Mark: marks[d], Adjusted: adjusted[d], Rest: !h.IsWorkDay(date.Weekday())}
		if s, ok := byDay[d]; ok {
			row.AMIn, row.AMOut = hhmm(s["am_in"]), hhmm(s["am_out"])
			row.PMIn, row.PMOut = hhmm(s["pm_in"]), hhmm(s["pm_out"])

			if row.Mark == "" && !row.Rest {
				row.UndertimeMin = undertime(s, date, h)
				row.Incomplete = len(s) < len(h.Slots()) && required(h) > 0
				if in := s["am_in"]; !in.IsZero() {
					row.Late = h.LateAt(mins(in, date))
				}
			}
			sheet.TotalUndertimeMin += row.UndertimeMin
		}
		sheet.Days = append(sheet.Days, row)
	}
	return sheet
}

func hhmm(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("15:04")
}

// mins counts from midnight of the work date, so an overnight shift keeps counting past 24:00.
func mins(t, date time.Time) int { return int(t.Sub(date).Minutes()) }

// clip keeps the part of a window before a suspension.
func clip(w Window, from int) Window {
	if from >= 0 && w.End > from {
		w.End = max(w.Start, from)
	}
	return w
}

// required is how many minutes are owed today, after any suspension.
func required(h Hours) int {
	am, pm := clip(h.AM, h.SuspendFrom), clip(h.PM, h.SuspendFrom)
	open := (am.End - am.Start) + (pm.End - pm.Start)
	if h.Flexi {
		return min(h.Required, open)
	}
	return open
}

// credit is the minutes of [in, out] inside the window. An arrival within the
// grace period counts from the start of the window.
func credit(in, out, date time.Time, w Window, grace int) int {
	if in.IsZero() || out.IsZero() {
		return 0
	}
	start := mins(in, date)
	if start <= w.Start+grace {
		start = w.Start
	}
	return max(0, min(mins(out, date), w.End)-max(start, w.Start))
}

func undertime(s map[string]time.Time, date time.Time, h Hours) int {
	am, pm := clip(h.AM, h.SuspendFrom), clip(h.PM, h.SuspendFrom)
	var worked int
	switch {
	case h.TwoLog:
		worked = credit(s["am_in"], s["pm_out"], date, am, h.Grace) + credit(s["am_in"], s["pm_out"], date, pm, h.Grace)
	default:
		worked = credit(s["am_in"], s["am_out"], date, am, h.Grace) + credit(s["pm_in"], s["pm_out"], date, pm, h.Grace)
	}
	return max(0, required(h)-worked)
}
