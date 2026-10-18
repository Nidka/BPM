package application

import (
	"context"
	"github.com/Nidka/BPM-service/internal/core/contracts"
	"time"
)

// State is keyed by verified UID + Firebase auth_time, never by client timestamps
// or the ID token's issue time (which changes during refresh).
type SessionRepository interface {
	CheckSession(context.Context, string, time.Time, time.Time, time.Duration, bool) (time.Time, error)
	EndSession(context.Context, string, time.Time, time.Time) error
}

type Sessions struct {
	repository     SessionRepository
	now            func() time.Time
	absolute, idle time.Duration
}

func NewSessions(repository SessionRepository, absolute, idle time.Duration, now func() time.Time) *Sessions {
	return &Sessions{repository: repository, absolute: absolute, idle: idle, now: now}
}

func (s *Sessions) Validate(ctx context.Context, identity contracts.Identity, activity bool) (contracts.SessionWindow, error) {
	now := s.now().UTC()
	start := identity.AuthenticatedAt
	if identity.UID == "" || start.IsZero() || start.Unix() <= 0 || start.After(now.Add(time.Minute)) || !now.Before(start.Add(s.absolute)) {
		return contracts.SessionWindow{}, contracts.ErrSessionExpired
	}
	last, err := s.repository.CheckSession(ctx, identity.UID, start, now, s.idle, activity)
	if err != nil {
		return contracts.SessionWindow{}, err
	}
	return contracts.SessionWindow{AbsoluteExpiresAt: start.Add(s.absolute), IdleExpiresAt: last.Add(s.idle)}, nil
}

func (s *Sessions) End(ctx context.Context, identity contracts.Identity) error {
	return s.repository.EndSession(ctx, identity.UID, identity.AuthenticatedAt, s.now().UTC())
}
