package application

import (
	"context"
	"github.com/Nidka/BPM-service/internal/features/checklists/domain"
	"github.com/Nidka/BPM-service/internal/shared/application/pagination"
	"time"
)

func (s *Service) History(ctx context.Context, uid, org string, request pagination.Request, filter domain.HistoryFilter) (out pagination.Page[domain.RunSummary], err error) {
	if filter.Scope == "" {
		filter.Scope = "mine"
	}
	if filter.Scope != "mine" && filter.Scope != "team" || filter.Status != "" && filter.Status != "DRAFT" && filter.Status != "SUBMITTED" {
		return out, pagination.ErrInvalid
	}
	scope := []string{uid, org, "checklist-history", filter.Scope, filter.Status}
	window, err := pagination.Open(request, scope...)
	if err != nil {
		return out, err
	}
	if window.After.ID != "" {
		if _, e := time.Parse(time.RFC3339Nano, window.After.Key); e != nil || !uuid.MatchString(window.After.ID) {
			return out, pagination.ErrInvalid
		}
	}
	err = s.work.Within(ctx, func(db Store) error {
		actor, e := db.Actor(ctx, uid, org)
		if e != nil {
			return e
		}
		if !domain.CanSuperviseRuns(actor.Role) {
			return domain.ErrDenied
		}
		items, e := db.RunHistory(ctx, actor, org, window, filter)
		if e != nil {
			return e
		}
		out = pagination.Close(items, window, func(v domain.RunSummary) pagination.Position {
			return pagination.Position{Key: v.CreatedAt.UTC().Format(time.RFC3339Nano), ID: v.ID}
		}, scope...)
		return nil
	})
	return
}
