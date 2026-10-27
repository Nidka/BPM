package domain

import (
	"testing"
	"time"
)

func TestAssignmentCalendarUsesLocationAndHalfOpenPeriods(t *testing.T) {
	for _, tc := range []struct{ frequency, zone, now, start, end string }{
		{"DAILY", "America/Lima", "2026-10-25T02:00:00Z", "2026-10-24T05:00:00Z", "2026-10-25T05:00:00Z"},
		{"WEEKLY", "America/Lima", "2026-10-25T02:00:00Z", "2026-10-19T05:00:00Z", "2026-10-26T05:00:00Z"},
		{"WEEKLY", "America/Lima", "2026-10-26T05:00:00Z", "2026-10-26T05:00:00Z", "2026-11-02T05:00:00Z"},
		{"MONTHLY", "America/Lima", "2026-03-01T03:00:00Z", "2026-02-01T05:00:00Z", "2026-03-01T05:00:00Z"},
		{"DAILY", "America/New_York", "2026-03-08T14:00:00Z", "2026-03-08T05:00:00Z", "2026-03-09T04:00:00Z"},
	} {
		t.Run(tc.frequency+tc.now, func(t *testing.T) {
			now, _ := time.Parse(time.RFC3339, tc.now)
			p, e := (Assignment{ID: "assignment", Frequency: tc.frequency, Timezone: tc.zone}).PeriodAt(now)
			if e != nil || p.StartsAt.Format(time.RFC3339) != tc.start || p.EndsAt == nil || p.EndsAt.Format(time.RFC3339) != tc.end {
				t.Fatalf("period=%+v error=%v", p, e)
			}
		})
	}
	created := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	p, e := (Assignment{Frequency: "ONCE", CreatedAt: created}).PeriodAt(time.Now())
	if e != nil || !p.StartsAt.Equal(created) || p.EndsAt != nil {
		t.Fatal("once unexpectedly repeats", p, e)
	}
}
