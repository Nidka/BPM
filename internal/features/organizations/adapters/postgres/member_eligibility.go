package postgres

import (
	"context"
	"errors"
	"github.com/Nidka/BPM-service/internal/features/organizations/domain"
	"github.com/jackc/pgx/v5"
)

func (s *store) MemberEligibility(ctx context.Context, org, email string) (domain.MemberEligibility, error) {
	// El perfil BPM se crea mediante identidad tras verificar el correo en Firebase.
	// Una cuenta inexistente o inactiva devuelve el mismo estado público.
	var assigned bool
	err := s.tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM organization_memberships m
        WHERE m.organization_id=$1 AND m.user_id=u.id)
        FROM application_users u WHERE LOWER(u.email)=LOWER($2) AND u.status='ACTIVE'`, org, email).Scan(&assigned)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.MemberUnavailable, nil
	}
	if err != nil {
		return "", err
	}
	if assigned {
		return domain.MemberAlreadyAssigned, nil
	}
	return domain.MemberReady, nil
}
