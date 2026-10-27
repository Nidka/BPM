package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Nidka/BPM-service/internal/features/checklists/domain"
	"github.com/Nidka/BPM-service/internal/shared/application/pagination"
)

type assignmentFake struct {
	actor     domain.Actor
	values    []domain.Assignment
	writes    int
	workCalls int
}

func (f *assignmentFake) WithinAssignments(ctx context.Context, fn func(AssignmentStore) error) error {
	f.workCalls++
	return fn(f)
}
func (f *assignmentFake) Actor(context.Context, string, string) (domain.Actor, error) {
	return f.actor, nil
}
func (f *assignmentFake) AssignmentPage(context.Context, domain.Actor, string, pagination.Window) ([]domain.Assignment, error) {
	return f.values, nil
}
func (f *assignmentFake) Assignment(context.Context, domain.Actor, string, string, bool) (domain.Assignment, error) {
	return f.values[0], nil
}
func (f *assignmentFake) CreateAssignment(_ context.Context, _ domain.Actor, _ string, a domain.Assignment) (domain.Assignment, error) {
	f.writes++
	return a, nil
}
func (f *assignmentFake) DeactivateAssignment(context.Context, domain.Actor, string, string) (domain.Assignment, error) {
	f.writes++
	return domain.Assignment{}, nil
}
func (f *assignmentFake) PeriodRuns(context.Context, string, []domain.AssignmentPeriod) (map[string]domain.AssignedRun, error) {
	return map[string]domain.AssignedRun{}, nil
}

func TestAssignmentValidationAndManagementPermissions(t *testing.T) {
	valid := domain.Assignment{ID: testRunID, TemplateID: "30000000-0000-0000-0000-000000000001", LocationID: "40000000-0000-0000-0000-000000000001", Frequency: "DAILY", DueTime: "08:30"}
	f := &assignmentFake{actor: domain.Actor{Role: "ADMIN"}}
	s := NewAssignments(f)
	for _, hour := range []string{"24:00", "8:30", "08:30:12", "08:30\n"} {
		v := valid
		v.DueTime = hour
		if _, e := s.Create(context.Background(), "a", "org", v); e == nil {
			t.Fatal("invalid hour accepted", hour)
		}
	}
	if f.workCalls != 0 {
		t.Fatal("invalid input consulted storage")
	}
	for _, role := range []string{"CHEF", "WORKER", "ADMIN", "SUPER_ADMIN"} {
		f.actor.Role = role
		_, err := s.Create(context.Background(), "a", "org", valid)
		if domain.CanManageTemplates(role) && err != nil || !domain.CanManageTemplates(role) && !errors.Is(err, domain.ErrDenied) {
			t.Fatal(role, err)
		}
	}
	if f.writes != 2 {
		t.Fatal("unauthorized write", f.writes)
	}
	f.actor.Role = "WORKER"
	if _, err := s.List(context.Background(), "w", "org", pagination.Request{}); !errors.Is(err, domain.ErrDenied) {
		t.Fatal("worker read assignments", err)
	}
}

func TestScheduledRunCannotDuplicatePeriodAndReplayRemainsPossible(t *testing.T) {
	id := "80000000-0000-0000-0000-000000000001"
	f := &fakeStore{actor: domain.Actor{ID: "chef", Role: "CHEF"}, schedule: domain.Assignment{ID: id, Frequency: "DAILY", Timezone: "America/Lima"}, periodRuns: map[string]domain.AssignedRun{id: {ID: "existing", Status: "SUBMITTED"}}}
	s := New(fakeWork{f})
	s.now = func() time.Time { return time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC) }
	v := domain.Run{ID: testRunID, AssignmentID: id, LocationID: "40000000-0000-0000-0000-000000000001", TemplateVersionID: "30000000-0000-0000-0000-000000000001", ClientOperationID: "50000000-0000-0000-0000-000000000001", ClientCreatedAt: s.now().Add(-time.Minute)}
	if _, err := s.CreateRun(context.Background(), "chef", "org", v); !errors.Is(err, domain.ErrAssignmentStarted) {
		t.Fatal("duplicate period accepted", err)
	}
	f.alreadyStarted = true
	if _, err := s.CreateRun(context.Background(), "chef", "org", v); err != nil {
		t.Fatal("replay blocked by current period", err)
	}
	f.alreadyStarted = false
	f.periodRuns = nil
	created, err := s.CreateRun(context.Background(), "chef", "org", v)
	if err != nil || !created.CreatedAt.Equal(s.now()) {
		t.Fatal("server timestamp not applied", created.CreatedAt, err)
	}
}
