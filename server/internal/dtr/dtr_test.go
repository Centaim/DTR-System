package dtr

import (
	"testing"
	"time"
)

var manila = time.FixedZone("PST", 8*3600)

func at(day, h, m int) time.Time { return time.Date(2026, 11, day, h, m, 0, 0, manila) }

func hours(t *testing.T) func(time.Time) Hours {
	h, err := ParseHours("08:00-12:00,13:00-17:00")
	if err != nil {
		t.Fatal(err)
	}
	return func(time.Time) Hours { return h }
}

func full(day int, in, lunchOut, lunchIn, out [2]int) []Log {
	return []Log{
		{Day: day, Slot: "am_in", At: at(day, in[0], in[1])},
		{Day: day, Slot: "am_out", At: at(day, lunchOut[0], lunchOut[1])},
		{Day: day, Slot: "pm_in", At: at(day, lunchIn[0], lunchIn[1])},
		{Day: day, Slot: "pm_out", At: at(day, out[0], out[1])},
	}
}

func TestOnTimeDayHasNoUndertime(t *testing.T) {
	s := Compute(2026, 11, full(2, [2]int{7, 55}, [2]int{12, 1}, [2]int{12, 58}, [2]int{17, 5}), nil, hours(t), manila)
	if d := s.Days[1]; d.UndertimeMin != 0 || d.Incomplete {
		t.Fatalf("got %+v", d)
	}
}

func TestLateAndEarlyCountAsUndertime(t *testing.T) {
	s := Compute(2026, 11, full(3, [2]int{8, 15}, [2]int{12, 0}, [2]int{13, 0}, [2]int{16, 30}), nil, hours(t), manila)
	if got := s.Days[2].UndertimeMin; got != 45 {
		t.Fatalf("want 45, got %d", got)
	}
	if s.TotalUndertimeMin != 45 {
		t.Fatalf("total %d", s.TotalUndertimeMin)
	}
}

func TestMissingDepartureIsFlaggedIncomplete(t *testing.T) {
	logs := full(4, [2]int{8, 0}, [2]int{12, 0}, [2]int{13, 0}, [2]int{17, 0})[:3]
	d := Compute(2026, 11, logs, nil, hours(t), manila).Days[3]
	if !d.Incomplete || d.UndertimeMin != 240 {
		t.Fatalf("got %+v", d)
	}
}

func TestDaysWithoutLogsAreBlank(t *testing.T) {
	s := Compute(2026, 11, nil, nil, hours(t), manila)
	if len(s.Days) != 30 || s.TotalUndertimeMin != 0 || s.Days[0].AMIn != "" {
		t.Fatalf("unexpected sheet: %d days, total %d", len(s.Days), s.TotalUndertimeMin)
	}
}

func TestParseHoursRejectsBadInput(t *testing.T) {
	for _, in := range []string{"", "08:00-12:00", "12:00-08:00,13:00-17:00", "08:00-12:00,11:00-17:00", "x-y,a-b"} {
		if _, err := ParseHours(in); err == nil {
			t.Errorf("%q accepted", in)
		}
	}
}

func TestScheduleChangeOnlyAffectsLaterDays(t *testing.T) {
	early, _ := ParseHours("08:00-12:00,13:00-17:00")
	late, _ := ParseHours("09:00-12:00,13:00-18:00")
	hf := func(d time.Time) Hours {
		if d.Day() >= 10 {
			return late
		}
		return early
	}
	logs := append(full(9, [2]int{8, 0}, [2]int{12, 0}, [2]int{13, 0}, [2]int{17, 0}),
		full(10, [2]int{8, 0}, [2]int{12, 0}, [2]int{13, 0}, [2]int{17, 0})...)
	s := Compute(2026, 11, logs, nil, hf, manila)
	if s.Days[8].UndertimeMin != 0 {
		t.Errorf("day 9 under old hours should be 0, got %d", s.Days[8].UndertimeMin)
	}
	// Day 10 hours are 9:00 to 18:00: credited 3h + 4h = 7h of 8h, so 60 minutes short.
	if s.Days[9].UndertimeMin != 60 {
		t.Errorf("day 10 want 60, got %d", s.Days[9].UndertimeMin)
	}
}

func TestApprovedLeaveExcusesUndertime(t *testing.T) {
	logs := full(3, [2]int{10, 0}, [2]int{12, 0}, [2]int{13, 0}, [2]int{15, 0})
	s := Compute(2026, 11, logs, map[int]string{3: "LEAVE", 5: "LEAVE"}, hours(t), manila)
	if d := s.Days[2]; d.UndertimeMin != 0 || d.Mark != "LEAVE" || d.Incomplete {
		t.Fatalf("day 3 with leave: %+v", d)
	}
	if d := s.Days[4]; d.Mark != "LEAVE" || d.AMIn != "" {
		t.Fatalf("day 5 leave without logs: %+v", d)
	}
	if s.TotalUndertimeMin != 0 {
		t.Fatalf("total %d", s.TotalUndertimeMin)
	}
}

func TestAdjustedLogIsFlagged(t *testing.T) {
	logs := full(4, [2]int{8, 0}, [2]int{12, 0}, [2]int{13, 0}, [2]int{17, 0})
	logs[3].Adjusted = true
	if d := Compute(2026, 11, logs, nil, hours(t), manila).Days[3]; !d.Adjusted || d.UndertimeMin != 0 {
		t.Fatalf("got %+v", d)
	}
}
