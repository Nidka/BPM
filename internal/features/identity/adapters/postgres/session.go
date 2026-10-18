package postgres

import (
	"context"
	"errors"
	"github.com/Nidka/BPM-service/internal/core/contracts"
	"github.com/jackc/pgx/v5"
	"time"
)

func (r *Repository) RequireSessionSchema(ctx context.Context) error {
	rows, err := r.pool.Query(ctx, `SELECT user_id,authenticated_at,last_activity_at,revoked FROM authentication_sessions LIMIT 0`)
	if err != nil {
		return errors.New("authentication session migration V2 is required")
	}
	rows.Close()
	return rows.Err()
}

func (r *Repository) CheckSession(ctx context.Context, uid string, start, now time.Time, idle time.Duration, activity bool) (time.Time, error) {
	var last time.Time
	// One atomic statement serializes concurrent checks across all replicas.
	// Expired/revoked contexts remain as tombstones: refresh cannot reopen them.
	err := r.pool.QueryRow(ctx, `INSERT INTO authentication_sessions(user_id,authenticated_at,last_activity_at)
 VALUES($1,$2,$3) ON CONFLICT(user_id,authenticated_at) DO UPDATE
 SET last_activity_at=CASE WHEN $5 THEN GREATEST(authentication_sessions.last_activity_at,$3) ELSE authentication_sessions.last_activity_at END
 WHERE NOT authentication_sessions.revoked AND authentication_sessions.last_activity_at > $3-($4 * interval '1 second')
 RETURNING last_activity_at`, uid, start, now, idle.Seconds(), activity).Scan(&last)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, contracts.ErrSessionExpired
	}
	return last, err
}

func (r *Repository) EndSession(ctx context.Context, uid string, start, now time.Time) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO authentication_sessions(user_id,authenticated_at,last_activity_at,revoked)
 VALUES($1,$2,$3,true) ON CONFLICT(user_id,authenticated_at) DO UPDATE SET revoked=true`, uid, start, now)
	return err
}
