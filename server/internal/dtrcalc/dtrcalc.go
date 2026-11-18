// Package dtrcalc gathers everything that shapes one employee's month, so the
// DTR and every report are always computed the same way.
package dtrcalc

import (
	"context"
	"time"

	"dtr/server/internal/attendance"
	"dtr/server/internal/dtr"
	"dtr/server/internal/holidays"
	"dtr/server/internal/requests"
	"dtr/server/internal/schedule"
)

type Service struct {
	att   *attendance.Service
	reqs  *requests.Store
	sched *schedule.Store
	hol   *holidays.Store
	loc   *time.Location
}

func New(att *attendance.Service, reqs *requests.Store, sched *schedule.Store, hol *holidays.Store, loc *time.Location) *Service {
	return &Service{att: att, reqs: reqs, sched: sched, hol: hol, loc: loc}
}

type Result struct {
	Sheet     dtr.Sheet
	HoursFor  func(time.Time) dtr.Hours
	Requested map[int]string // days excused by an approved request
}

const suspendedMark = "WORK SUSPENDED"

func (s *Service) Month(ctx context.Context, employeeID int64, office string, y int, m time.Month) (Result, error) {
	logs, err := s.att.MonthLogs(ctx, employeeID, y, m, s.loc)
	if err != nil {
		return Result{}, err
	}
	base, err := s.sched.HoursFunc(ctx, employeeID)
	if err != nil {
		return Result{}, err
	}
	adj, requested, err := s.reqs.ForMonth(ctx, employeeID, y, m)
	if err != nil {
		return Result{}, err
	}
	hol, err := s.hol.ForMonth(ctx, y, m)
	if err != nil {
		return Result{}, err
	}
	list, err := s.hol.Suspensions(ctx, y, m)
	if err != nil {
		return Result{}, err
	}

	marks := map[int]string{}
	for d, l := range hol {
		marks[d] = l
	}
	for d, l := range requested {
		marks[d] = l
	}

	partial := map[string]int{} // date to minute work stopped
	for _, x := range list {
		if x.Office != "" && x.Office != office {
			continue
		}
		day, err := time.ParseInLocation("2006-01-02", x.Day, s.loc)
		if err != nil {
			continue
		}
		if x.From == "" {
			if _, taken := marks[day.Day()]; !taken {
				marks[day.Day()] = suspendedMark
			}
			continue
		}
		if at, err := dtr.ParseMinutes(x.From); err == nil {
			partial[x.Day] = at
		}
	}
	hoursFor := func(d time.Time) dtr.Hours {
		h := base(d)
		if at, ok := partial[d.Format("2006-01-02")]; ok {
			h.SuspendFrom = at
		}
		return h
	}
	sheet := dtr.Compute(y, m, append(logs, adj...), marks, hoursFor, s.loc)
	return Result{Sheet: sheet, HoursFor: hoursFor, Requested: requested}, nil
}
