package application

import (
	"context"
	"errors"
	"github.com/Nidka/BPM-service/internal/core/contracts"
	"testing"
	"time"
)

type sessionStub struct {
	calls int
	last  time.Time
	err   error
}

func (s *sessionStub) CheckSession(_ context.Context, _ string, _, now time.Time, _ time.Duration, activity bool) (time.Time, error) {
	s.calls++
	return s.last, s.err
}
func (s *sessionStub) EndSession(context.Context, string, time.Time, time.Time) error { return s.err }

func TestAbsoluteSessionUsesVerifiedAuthenticationTime(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name    string
		start   time.Time
		expired bool
	}{
		{"just before limit", now.Add(-8*time.Hour + time.Second), false},
		{"exact boundary", now.Add(-8 * time.Hour), true},
		{"old login with newly refreshed token", now.Add(-9 * time.Hour), true},
		{"missing auth_time", time.Time{}, true},
		{"epoch auth_time", time.Unix(0, 0), true},
		{"future auth_time", now.Add(2 * time.Minute), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &sessionStub{last: now}
			sessions := NewSessions(repo, 8*time.Hour, 15*time.Minute, func() time.Time { return now })
			window, err := sessions.Validate(context.Background(), contracts.Identity{UID: "person", AuthenticatedAt: tc.start}, true)
			if tc.expired {
				if !errors.Is(err, contracts.ErrSessionExpired) || repo.calls != 0 {
					t.Fatal("expired context reached storage", err)
				}
			} else if err != nil || !window.AbsoluteExpiresAt.Equal(tc.start.Add(8*time.Hour)) || !window.IdleExpiresAt.Equal(now.Add(15*time.Minute)) {
				t.Fatal("incorrect window", window, err)
			}
		})
	}
}

func TestSessionStoreFailureDoesNotGrantAccess(t *testing.T) {
	now := time.Now()
	repo := &sessionStub{err: errors.New("private database failure")}
	sessions := NewSessions(repo, 8*time.Hour, 15*time.Minute, func() time.Time { return now })
	if _, err := sessions.Validate(context.Background(), contracts.Identity{UID: "person", AuthenticatedAt: now}, true); err == nil {
		t.Fatal("storage failure allowed session")
	}
}
