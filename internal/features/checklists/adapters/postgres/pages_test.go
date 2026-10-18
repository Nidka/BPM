package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Nidka/BPM-service/internal/core/contracts"
	"github.com/Nidka/BPM-service/internal/core/httpserver/middleware"
	checklisthttp "github.com/Nidka/BPM-service/internal/features/checklists/adapters/http"
	"github.com/Nidka/BPM-service/internal/features/checklists/application"
	"github.com/Nidka/BPM-service/internal/features/checklists/domain"
	"github.com/Nidka/BPM-service/internal/shared/application/pagination"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"
)

func testCatalogPagination(t *testing.T, ctx context.Context, pool *pgxpool.Pool, service *application.Service, org, other string) {
	t.Helper()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := pool.Exec(ctx, q, args...); e != nil {
			t.Fatal(e)
		}
	}
	exec(`INSERT INTO checklist_templates(id,organization_id,name,area) SELECT md5('template-'||n)::uuid,$1,'Plantilla '||lpad(n::text,3,'0'),'Cocina' FROM generate_series(1,137)n`, org)
	exec(`INSERT INTO checklist_template_versions(id,organization_id,template_id,version_number,status,published_at) SELECT md5('version-'||n)::uuid,$1,md5('template-'||n)::uuid,1,'PUBLISHED',NOW() FROM generate_series(1,137)n`, org)
	exec(`INSERT INTO checklist_items(id,organization_id,template_version_id,section_name,label,response_type,display_order,required) SELECT md5('item-'||n||'-'||i)::uuid,$1,md5('version-'||n)::uuid,'Personal','Pregunta '||i,'COMPLIANCE',i,true FROM generate_series(1,137)n CROSS JOIN generate_series(1,100)i`, org)
	ids := map[string]bool{}
	next := ""
	for {
		p, e := service.CatalogPage(ctx, "worker", org, pagination.Request{Size: 17, Query: "Plantilla", Token: next})
		if e != nil {
			t.Fatal(e)
		}
		for _, v := range p.Items {
			if ids[v.ID] || len(v.Items) != 100 {
				t.Fatal("duplicated or split template", v.ID, len(v.Items))
			}
			ids[v.ID] = true
		}
		next = p.NextToken
		if next == "" {
			break
		}
		if len(ids) > 150 {
			t.Fatal("endless")
		}
	}
	if len(ids) != 137 {
		t.Fatal("catalog cutoff", len(ids))
	}
	// El detalle no depende de que la plantilla esté en la primera página del catálogo.
	var id string
	if e := pool.QueryRow(ctx, `SELECT id::text FROM checklist_templates WHERE organization_id=$1 AND name='Plantilla 137'`, org).Scan(&id); e != nil {
		t.Fatal(e)
	}
	detail, e := service.Template(ctx, "worker", org, id)
	if e != nil || len(detail.Items) != 100 {
		t.Fatal("detail unavailable", e)
	}
	if _, e = service.Template(ctx, "outsider", org, id); !errors.Is(e, domain.ErrDenied) {
		t.Fatal("foreign detail", e)
	}
	if _, e = service.Template(ctx, "outsider", other, id); !errors.Is(e, domain.ErrNotFound) {
		t.Fatal("template crossed tenant", e)
	}
	p, e := service.CatalogPage(ctx, "worker", org, pagination.Request{Size: 1})
	if e != nil {
		t.Fatal(e)
	}
	router := chi.NewRouter()
	checklisthttp.New(service, slog.New(slog.NewTextHandler(io.Discard, nil))).Register(router, middleware.Authenticate(pageVerifier{}))
	for _, test := range []struct {
		path   string
		status int
	}{{"/checklists?pageSize=3&query=Plantilla", 200}, {"/checklists/" + id, 200}, {"/checklists?pageSize=101", 400}, {"/checklists/" + id + "?pageSize=1", 400}} {
		request := httptest.NewRequest("GET", "/organizations/"+org+test.path, nil)
		request.Header.Set("Authorization", "Bearer test")
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		if recorder.Code != test.status {
			t.Fatal("HTTP catalog contract", test.path, recorder.Code, recorder.Body.String())
		}
		if test.status == 200 {
			var body map[string]json.RawMessage
			if json.Unmarshal(recorder.Body.Bytes(), &body) != nil || body["data"] == nil {
				t.Fatal("missing data")
			}
		}
	}

	exec(`UPDATE organization_memberships SET status='INACTIVE' WHERE organization_id=$1 AND user_id='worker'`, org)
	if _, e = service.CatalogPage(ctx, "worker", org, pagination.Request{Token: p.NextToken}); !errors.Is(e, domain.ErrDenied) {
		t.Fatal("cursor bypassed revocation", e)
	}
}

type pageVerifier struct{}

func (pageVerifier) VerifyIDToken(context.Context, string) (contracts.Identity, error) {
	return contracts.Identity{UID: "worker", EmailVerified: true}, nil
}
