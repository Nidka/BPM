package postgres

import (
	"context"
	"errors"
	"github.com/Nidka/BPM-service/internal/features/checklists/application"
	"github.com/Nidka/BPM-service/internal/features/checklists/domain"
	"github.com/Nidka/BPM-service/internal/shared/application/pagination"
	"github.com/jackc/pgx/v5/pgxpool"
	"testing"
)

func testRunHistory(t *testing.T, ctx context.Context, pool *pgxpool.Pool, service *application.Service, org, other, loc, version, item string) {
	t.Helper()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := pool.Exec(ctx, q, args...); e != nil {
			t.Fatal(e)
		}
	}
	exec(`INSERT INTO checklist_runs(id,organization_id,location_id,template_version_id,performed_by,client_operation_id,client_created_at,created_at,status,submitted_at,compliance_percent)
 SELECT md5('history-'||n)::uuid,$1,$2,$3,'chef',md5('history-op-'||n)::uuid,NOW(),TIMESTAMPTZ '2026-01-01 12:00:00Z'+(n/3)*interval '1 second',
 CASE WHEN n%2=0 THEN 'SUBMITTED' ELSE 'DRAFT' END,CASE WHEN n%2=0 THEN NOW() END,CASE WHEN n%2=0 THEN 100 END FROM generate_series(1,137)n`, org, loc, version)
	exec(`INSERT INTO checklist_answers(organization_id,checklist_run_id,checklist_item_id,compliance_result) SELECT $1,md5('history-'||n)::uuid,$2,'COMPLIANT' FROM generate_series(1,137)n WHERE n%2=0`, org, item)
	const hidden = "99000000-0000-0000-0000-000000000001"
	exec(`INSERT INTO locations(id,organization_id,name,address)VALUES($1,$2,'Sede no asignada','Dirección de prueba')`, hidden, org)
	exec(`INSERT INTO checklist_runs(id,organization_id,location_id,template_version_id,performed_by,client_operation_id,client_created_at)VALUES(md5('hidden-history')::uuid,$1,$2,$3,'admin',md5('hidden-history-op')::uuid,NOW())`, org, hidden, version)
	if _, e := service.History(ctx, "worker", org, pagination.Request{}, domain.HistoryFilter{Scope: "team"}); !errors.Is(e, domain.ErrDenied) {
		t.Fatal("worker supervised team", e)
	}
	if _, e := service.History(ctx, "worker", org, pagination.Request{}, domain.HistoryFilter{Scope: "mine"}); !errors.Is(e, domain.ErrDenied) {
		t.Fatal("worker accessed checklist history", e)
	}
	if _, e := service.History(ctx, "outsider", org, pagination.Request{}, domain.HistoryFilter{}); !errors.Is(e, domain.ErrDenied) {
		t.Fatal("foreign tenant history", e)
	}
	ids := map[string]bool{}
	token := ""
	for {
		page, e := service.History(ctx, "chef", org, pagination.Request{Size: 17, Token: token}, domain.HistoryFilter{Scope: "team"})
		if e != nil {
			t.Fatal(e)
		}
		for _, v := range page.Items {
			if ids[v.ID] || v.LocationID == hidden || v.TemplateName == "" || v.CreatedAt.IsZero() {
				t.Fatal("invalid or unauthorized summary", v.ID)
			}
			ids[v.ID] = true
		}
		token = page.NextToken
		if token == "" {
			break
		}
		if len(ids) > 150 {
			t.Fatal("endless history")
		}
	}
	if len(ids) != 138 {
		t.Fatal("history cutoff or duplicates", len(ids))
	}
	for _, uid := range []string{"admin", "super"} {
		page, e := service.History(ctx, uid, org, pagination.Request{Size: 100}, domain.HistoryFilter{Scope: "team"})
		if e != nil || len(page.Items) != 100 {
			t.Fatal("supervision unavailable", uid, e)
		}
	}
	page, e := service.History(ctx, "chef", org, pagination.Request{Size: 3}, domain.HistoryFilter{Status: "DRAFT"})
	if e != nil || len(page.Items) != 3 || page.NextToken == "" {
		t.Fatal("draft history", e)
	}
	for _, v := range page.Items {
		if v.Status != "DRAFT" {
			t.Fatal("status not filtered")
		}
	}
	if _, e = service.History(ctx, "chef", org, pagination.Request{Token: page.NextToken}, domain.HistoryFilter{Status: "SUBMITTED"}); !errors.Is(e, pagination.ErrInvalid) {
		t.Fatal("cursor crossed status", e)
	}
	if _, e = service.History(ctx, "chef", other, pagination.Request{Token: page.NextToken}, domain.HistoryFilter{Status: "DRAFT"}); !errors.Is(e, pagination.ErrInvalid) {
		t.Fatal("cursor crossed tenant", e)
	}
	exec(`UPDATE organization_memberships SET status='INACTIVE' WHERE organization_id=$1 AND user_id='chef'`, org)
	if _, e = service.History(ctx, "chef", org, pagination.Request{Token: page.NextToken}, domain.HistoryFilter{Status: "DRAFT"}); !errors.Is(e, domain.ErrDenied) {
		t.Fatal("cursor bypassed revocation", e)
	}
	exec(`UPDATE organization_memberships SET status='ACTIVE' WHERE organization_id=$1 AND user_id='chef'`, org)
}
