package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Nidka/BPM-service/internal/features/checklists/application"
	"github.com/Nidka/BPM-service/internal/features/checklists/domain"
	"github.com/Nidka/BPM-service/internal/shared/application/pagination"
	"github.com/jackc/pgx/v5/pgxpool"
)

func testAssignments(t *testing.T, ctx context.Context, pool *pgxpool.Pool, runs *application.Service, org, other, loc, template, version, item string) {
	t.Helper()
	service := application.NewAssignments(New(pool))
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := pool.Exec(ctx, q, args...); e != nil {
			t.Fatal(e)
		}
	}
	input := domain.Assignment{ID: "81000000-0000-0000-0000-000000000001", TemplateID: template, LocationID: loc, Frequency: "DAILY", DueTime: "09:30"}
	for _, uid := range []string{"chef", "worker", "outsider"} {
		if _, e := service.Create(ctx, uid, org, input); !errors.Is(e, domain.ErrDenied) {
			t.Fatal("unauthorized assignment", uid, e)
		}
	}
	a, e := service.Create(ctx, "admin", org, input)
	if e != nil || !a.Active || a.Timezone != "America/Lima" {
		t.Fatal("assignment creation", a, e)
	}
	if _, e = service.Create(ctx, "admin", org, input); e != nil {
		t.Fatal("idempotent creation", e)
	}
	duplicate := input
	duplicate.ID = "81000000-0000-0000-0000-000000000002"
	if _, e = service.Create(ctx, "admin", org, duplicate); !errors.Is(e, domain.ErrConflict) {
		t.Fatal("duplicate active assignment", e)
	}
	foreign := input
	foreign.ID = duplicate.ID
	foreign.LocationID = "ffffffff-0000-0000-0000-000000000001"
	if _, e = service.Create(ctx, "admin", org, foreign); !errors.Is(e, domain.ErrNotFound) {
		t.Fatal("foreign location", e)
	}
	if _, e = service.Get(ctx, "outsider", org, input.ID); !errors.Is(e, domain.ErrDenied) {
		t.Fatal("foreign tenant read", e)
	}
	if _, e = service.List(ctx, "worker", org, pagination.Request{}); !errors.Is(e, domain.ErrDenied) {
		t.Fatal("worker listed assignments", e)
	}
	v := domain.Run{ID: "82000000-0000-0000-0000-000000000001", LocationID: loc, TemplateVersionID: version, ClientOperationID: "83000000-0000-0000-0000-000000000001", ClientCreatedAt: time.Now().AddDate(0, 0, -5)}
	if _, e = runs.CreateRun(ctx, "chef", org, v); !errors.Is(e, domain.ErrAssignmentRequired) {
		t.Fatal("scheduled review bypassed", e)
	}
	v.AssignmentID = input.ID
	started, e := runs.CreateRun(ctx, "chef", org, v)
	if e != nil || started.AssignmentID != input.ID || started.CreatedAt.Before(time.Now().Add(-time.Minute)) {
		t.Fatal("scheduled start", e)
	}
	if _, e = runs.CreateRun(ctx, "chef", org, v); e != nil {
		t.Fatal("scheduled replay", e)
	}
	second := v
	second.ID = "82000000-0000-0000-0000-000000000002"
	second.ClientOperationID = "83000000-0000-0000-0000-000000000002"
	if _, e = runs.CreateRun(ctx, "chef", org, second); !errors.Is(e, domain.ErrAssignmentStarted) {
		t.Fatal("client backdate bypassed period", e)
	}
	detail, e := service.Get(ctx, "chef", org, input.ID)
	if e != nil || detail.Run == nil || detail.Run.ID != v.ID || detail.Run.Status != "DRAFT" {
		t.Fatal("draft not resumable", detail, e)
	}
	// La asignación retirada no invalida respuestas de una revisión ya iniciada.
	if _, e = service.Deactivate(ctx, "admin", org, input.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = service.Deactivate(ctx, "admin", org, input.ID); e != nil {
		t.Fatal("idempotent deactivate", e)
	}
	if _, e = runs.CreateRun(ctx, "chef", org, v); e != nil {
		t.Fatal("deactivation broke replay", e)
	}
	if _, e = runs.SaveAnswers(ctx, "chef", org, v.ID, 1, "assignment-save-1", []domain.Answer{{ItemID: item, ComplianceResult: "COMPLIANT"}}); e != nil {
		t.Fatal("deactivation broke existing draft", e)
	}
	var audits int
	if e = pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE organization_id=$1 AND entity_id=$2 AND entity_type='checklist_assignment'`, org, input.ID).Scan(&audits); e != nil || audits != 2 {
		t.Fatal("duplicate audit", audits, e)
	}
	// Más de una página y una sede ajena a las autorizadas del chef.
	exec(`INSERT INTO checklist_assignments(id,organization_id,location_id,template_id,frequency)
	 SELECT md5('assignment-page-'||t.id::text)::uuid,$1::varchar(64),$2::uuid,t.id,'WEEKLY' FROM checklist_templates t WHERE t.organization_id=$1::varchar(64) AND t.id<>$3::uuid`, org, loc, template)
	hidden := "99000000-0000-0000-0000-000000000001"
	hiddenInput := input
	hiddenInput.ID = duplicate.ID
	hiddenInput.LocationID = hidden
	if _, e = service.Create(ctx, "super", org, hiddenInput); e != nil {
		t.Fatal("super assignment", e)
	}
	if _, e = service.Get(ctx, "chef", org, hiddenInput.ID); !errors.Is(e, domain.ErrNotFound) {
		t.Fatal("chef accessed foreign site assignment", e)
	}
	ids := map[string]bool{}
	token := ""
	var concurrent domain.Assignment
	for {
		page, err := service.List(ctx, "chef", org, pagination.Request{Size: 17, Token: token})
		if err != nil {
			t.Fatal(err)
		}
		for _, x := range page.Items {
			if ids[x.ID] || x.LocationID != loc || x.ID == input.ID || x.Period.EndsAt == nil {
				t.Fatal("invalid page", x)
			}
			ids[x.ID] = true
			concurrent = x
		}
		token = page.NextToken
		if token == "" {
			break
		}
		if len(ids) > 200 {
			t.Fatal("endless pagination")
		}
	}
	if len(ids) != 137 {
		t.Fatal("pagination cutoff", len(ids))
	}
	page, e := service.List(ctx, "chef", org, pagination.Request{Size: 1})
	if e != nil || page.NextToken == "" {
		t.Fatal("cursor missing", e)
	}
	if _, e = service.List(ctx, "chef", other, pagination.Request{Size: 1, Token: page.NextToken}); !errors.Is(e, pagination.ErrInvalid) {
		t.Fatal("cursor crossed tenant", e)
	}
	// Dos clientes compiten por el mismo período: el bloqueo permite una sola alta.
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func(i int) {
			id := []string{"84000000-0000-0000-0000-000000000001", "84000000-0000-0000-0000-000000000002"}[i]
			_, err := runs.CreateRun(ctx, "chef", org, domain.Run{ID: id, AssignmentID: concurrent.ID, LocationID: loc, TemplateVersionID: concurrent.TemplateVersionID, ClientOperationID: id, ClientCreatedAt: time.Now()})
			results <- err
		}(i)
	}
	success, conflict := 0, 0
	for i := 0; i < 2; i++ {
		err := <-results
		if err == nil {
			success++
		} else if errors.Is(err, domain.ErrAssignmentStarted) {
			conflict++
		} else {
			t.Fatal("concurrent start", err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal("period duplicate", success, conflict)
	}
	exec(`UPDATE organization_memberships SET status='INACTIVE' WHERE organization_id=$1 AND user_id='chef'`, org)
	if _, e = service.List(ctx, "chef", org, pagination.Request{Token: page.NextToken}); !errors.Is(e, domain.ErrDenied) {
		t.Fatal("revocation bypassed", e)
	}
	exec(`UPDATE organization_memberships SET status='ACTIVE' WHERE organization_id=$1 AND user_id='chef'`, org)
}
