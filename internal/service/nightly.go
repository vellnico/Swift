package service

import (
	"context"
	"time"
)

// nightHour is when nightly jobs start, in the server's local time.
const nightHour = 3

// NextNight returns the first moment after now at nightHour:00 plus offset.
func NextNight(now time.Time, offset time.Duration) time.Time {
	day := time.Date(now.Year(), now.Month(), now.Day(), nightHour, 0, 0, 0, now.Location())
	if t := day.Add(offset); t.After(now) {
		return t
	}
	return day.AddDate(0, 0, 1).Add(offset)
}

// runNightly calls job once a night until ctx ends. A check older than a day
// (a fresh install, a server that was off) runs right away with night=false,
// so updates are found early but installed only at night.
func runNightly(ctx context.Context, offset time.Duration, lastRun func() time.Time, job func(night bool)) {
	if time.Since(lastRun()) > 26*time.Hour {
		job(false)
	}
	for {
		t := time.NewTimer(time.Until(NextNight(time.Now(), offset)))
		select {
		case <-ctx.Done():
			t.Stop()
			return
		case <-t.C:
			job(true)
		}
	}
}
