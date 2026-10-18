package application

import (
	"context"

	"github.com/Nidka/BPM-service/internal/features/checklists/domain"
	"github.com/Nidka/BPM-service/internal/shared/application/pagination"
)

func (s *Service) CatalogPage(ctx context.Context, uid, org string, r pagination.Request) (out pagination.Page[domain.Template], err error) {
	scope := []string{uid, org, "published-checklists"}
	w, err := pagination.Open(r, scope...)
	if err != nil {
		return out, err
	}
	err = s.work.Within(ctx, func(db Store) error {
		if _, e := db.Actor(ctx, uid, org); e != nil {
			return e
		}
		items, e := db.CatalogPage(ctx, org, w, "")
		if e != nil {
			return e
		}
		out = pagination.Close(items, w, func(t domain.Template) pagination.Position { return pagination.Position{Key: t.Name, ID: t.ID} }, scope...)
		return nil
	})
	return
}
func (s *Service) Template(ctx context.Context, uid, org, id string) (out domain.Template, err error) {
	if !uuid.MatchString(id) {
		return out, domain.ErrNotFound
	}
	err = s.work.Within(ctx, func(db Store) error {
		if _, e := db.Actor(ctx, uid, org); e != nil {
			return e
		}
		items, e := db.CatalogPage(ctx, org, pagination.Window{Size: 1}, id)
		if e != nil {
			return e
		}
		if len(items) == 0 {
			return domain.ErrNotFound
		}
		out = items[0]
		return nil
	})
	return
}
