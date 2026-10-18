package application

import (
	"context"

	"github.com/Nidka/BPM-service/internal/features/organizations/domain"
	"github.com/Nidka/BPM-service/internal/shared/application/pagination"
)

func (s *Service) OrganizationsPage(ctx context.Context, uid string, request pagination.Request) (out pagination.Page[domain.Organization], err error) {
	scope := []string{uid, "organizations"}
	w, err := pagination.Open(request, scope...)
	if err != nil {
		return out, err
	}
	err = s.work.Within(ctx, func(db Store) error {
		actor, e := db.Actor(ctx, uid)
		if e != nil {
			return e
		}
		items, e := db.OrganizationsPage(ctx, actor, w)
		if e != nil {
			return e
		}
		out = pagination.Close(items, w, func(v domain.Organization) pagination.Position {
			return pagination.Position{Key: v.LegalName, ID: v.ID}
		}, scope...)
		return nil
	})
	return
}

func authorizedPage[T any](ctx context.Context, s *Service, uid, org, resource string, request pagination.Request, read func(Store, domain.Actor, string, pagination.Window) ([]T, error), position func(T) pagination.Position) (out pagination.Page[T], err error) {
	scope := []string{uid, org, resource}
	w, err := pagination.Open(request, scope...)
	if err != nil {
		return out, err
	}
	err = s.work.Within(ctx, func(db Store) error {
		actor, e := db.Actor(ctx, uid)
		if e != nil {
			return e
		}
		role, e := db.Role(ctx, actor, org)
		if e != nil {
			return e
		}
		if resource == "members" && !domain.CanManage(role) {
			return domain.ErrDenied
		}
		items, e := read(db, actor, role, w)
		if e != nil {
			return e
		}
		out = pagination.Close(items, w, position, scope...)
		return nil
	})
	return
}
func (s *Service) LocationsPage(ctx context.Context, uid, org string, r pagination.Request) (pagination.Page[domain.Location], error) {
	return authorizedPage(ctx, s, uid, org, "locations", r, func(db Store, a domain.Actor, role string, w pagination.Window) ([]domain.Location, error) {
		return db.LocationsPage(ctx, a, org, role, w)
	}, func(v domain.Location) pagination.Position { return pagination.Position{Key: v.Name, ID: v.ID} })
}
func (s *Service) MembersPage(ctx context.Context, uid, org string, r pagination.Request) (pagination.Page[domain.Member], error) {
	return authorizedPage(ctx, s, uid, org, "members", r, func(db Store, _ domain.Actor, _ string, w pagination.Window) ([]domain.Member, error) {
		return db.MembersPage(ctx, org, w)
	}, func(v domain.Member) pagination.Position { return pagination.Position{Key: v.Name, ID: v.ID} })
}
func (s *Service) EntriesPage(ctx context.Context, uid, org, resource string, r pagination.Request) (pagination.Page[domain.Entry], error) {
	if resource != "courses" && resource != "checklist-summaries" && resource != "incidents" {
		return pagination.Page[domain.Entry]{}, pagination.ErrInvalid
	}
	return authorizedPage(ctx, s, uid, org, resource, r, func(db Store, a domain.Actor, role string, w pagination.Window) ([]domain.Entry, error) {
		return db.EntriesPage(ctx, a, org, role, resource, w)
	}, func(v domain.Entry) pagination.Position {
		return pagination.Position{Key: v.CursorKey, ID: v.ID}
	})
}

func (s *Service) WorkspacePage(ctx context.Context, uid, org string, size int) (out domain.Workspace, err error) {
	w, e := pagination.Open(pagination.Request{Size: size})
	if e != nil {
		return out, e
	}
	err = s.work.Within(ctx, func(db Store) error {
		a, e := db.Actor(ctx, uid)
		if e != nil {
			return e
		}
		role, e := db.Role(ctx, a, org)
		if e != nil {
			return e
		}
		out, e = db.WorkspacePage(ctx, a, org, role, w.Size)
		return e
	})
	return
}
