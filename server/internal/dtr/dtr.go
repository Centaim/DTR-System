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

// Hours are the official morning and afternoon windows.
type Hours struct{ AM, PM Window }

func (h Hours) Total() int { return (h.AM.End - h.AM.Start) + (h.PM.End - h.PM.Start) }

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
	return Hours{AM: w[0], PM: w[1]}, nil
}

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
	Day  int
	Slot string
	At   time.Time
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
}

type Sheet struct {
	Month             string `json:"month"` // YYYY-MM
	Days              []Day  `json:"days"`
	TotalUndertimeMin int    `json:"total_undertime_min"`
}

// Compute builds the sheet. hoursFor gives the official hours in force on a date,
// so a schedule change never rewrites earlier days. Undertime counts late arrival, early departure and
// unrecorded time inside official hours, for days that have at least one log.
// A day missing a pair is flagged Incomplete so a person reviews it, because
// the figure for that day is then an upper bound.
func Compute(year int, month time.Month, logs []Log, hoursFor func(time.Time) Hours, loc *time.Location) Sheet {
	byDay := map[int]map[string]time.Time{}
	for _, l := range logs {
		if byDay[l.Day] == nil {
			byDay[l.Day] = map[string]time.Time{}
		}
		byDay[l.Day][l.Slot] = l.At
	}

	first := time.Date(year, month, 1, 0, 0, 0, 0, loc)
	n := first.AddDate(0, 1, -1).Day()
	sheet := Sheet{Month: first.Format("2006-01")}

	for d := 1; d <= n; d++ {
		date := time.Date(year, month, d, 0, 0, 0, 0, loc)
		h := hoursFor(date)
		row := Day{Day: d, Weekday: date.Weekday().String()}
		if s, ok := byDay[d]; ok {
			row.AMIn, row.AMOut = hhmm(s["am_in"]), hhmm(s["am_out"])
			row.PMIn, row.PMOut = hhmm(s["pm_in"]), hhmm(s["pm_out"])

			credited := overlap(s["am_in"], s["am_out"], h.AM) + overlap(s["pm_in"], s["pm_out"], h.PM)
			row.UndertimeMin = max(0, h.Total()-credited)
			row.Incomplete = len(s) < 4
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

func mins(t time.Time) int { return t.Hour()*60 + t.Minute() }

// overlap is the credited minutes of [in, out] inside the window.
func overlap(in, out time.Time, w Window) int {
	if in.IsZero() || out.IsZero() {
		return 0
	}
	start, end := max(mins(in), w.Start), min(mins(out), w.End)
	return max(0, end-start)
}
