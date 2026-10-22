package postgres

import (
	"context"
	"github.com/Nidka/BPM-service/internal/features/checklists/domain"
	"github.com/Nidka/BPM-service/internal/shared/application/pagination"
	"time"
)

// RunHistory aplica el mismo acceso por sede del detalle y vuelve a autorizar cada página.
func (s *store) RunHistory(ctx context.Context, actor domain.Actor, org string, w pagination.Window, filter domain.HistoryFilter) ([]domain.RunSummary, error) {
	var anchor any
	if w.After.ID != "" {
		value, e := time.Parse(time.RFC3339Nano, w.After.Key)
		if e != nil {
			return nil, pagination.ErrInvalid
		}
		anchor = value
	}
	rows, err := s.tx.Query(ctx, `SELECT r.id::text,t.name,l.id::text,l.name,r.performed_by,u.display_name,r.status,r.created_at,r.submitted_at,r.compliance_percent::text,
 (SELECT count(*) FROM checklist_answers ca WHERE ca.organization_id=r.organization_id AND ca.checklist_run_id=r.id),
 (SELECT count(*) FROM checklist_items ci WHERE ci.organization_id=r.organization_id AND ci.template_version_id=r.template_version_id)
 FROM checklist_runs r
 JOIN locations l ON l.organization_id=r.organization_id AND l.id=r.location_id AND l.status='ACTIVE'
 JOIN checklist_template_versions v ON v.organization_id=r.organization_id AND v.id=r.template_version_id
 JOIN checklist_templates t ON t.organization_id=v.organization_id AND t.id=v.template_id
 JOIN application_users u ON u.id=r.performed_by
 WHERE r.organization_id=$1 AND r.status IN ('DRAFT','SUBMITTED')
 AND ($3='SUPER_ADMIN' OR EXISTS(SELECT 1 FROM organization_memberships m WHERE m.organization_id=r.organization_id AND m.user_id=$2 AND m.status='ACTIVE' AND
 (m.role='ADMIN' OR EXISTS(SELECT 1 FROM membership_locations ml WHERE ml.organization_id=m.organization_id AND ml.user_id=m.user_id AND ml.location_id=r.location_id))))
 AND ($4='team' OR r.performed_by=$2) AND ($5='' OR r.status=$5)
 AND ($6='' OR strpos(lower(t.name||' '||l.name||' '||u.display_name),lower($6))>0)
 AND ($7::timestamptz IS NULL OR (r.created_at,r.id)<($7::timestamptz,$8::uuid))
 ORDER BY r.created_at DESC,r.id DESC LIMIT $9`, org, actor.ID, actor.Role, filter.Scope, filter.Status, w.Query, anchor, nullableID(w.After.ID), w.Size+1)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.RunSummary{}
	for rows.Next() {
		var v domain.RunSummary
		var score *string
		if err = rows.Scan(&v.ID, &v.TemplateName, &v.LocationID, &v.LocationName, &v.PerformedBy, &v.PerformedByName, &v.Status, &v.CreatedAt, &v.SubmittedAt, &score, &v.AnswerCount, &v.QuestionCount); err != nil {
			return nil, err
		}
		if v.CompliancePercent, err = parseNullable(score); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func nullableID(id string) any {
	if id == "" {
		return nil
	}
	return id
}
