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

func (s *store) MemberAccountSuggestions(ctx context.Context, org, prefix string) ([]string, error) {
	// strpos trata %, _ y otros caracteres como texto; no se publica un directorio paginado.
	rows, err := s.tx.Query(ctx, `SELECT u.email FROM application_users u
        WHERE u.status='ACTIVE' AND strpos(LOWER(u.email), $2)=1
          AND NOT EXISTS (SELECT 1 FROM organization_memberships m WHERE m.organization_id=$1 AND m.user_id=u.id)
        ORDER BY LOWER(u.email), u.id LIMIT 5`, org, prefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []string{}
	for rows.Next() {
		var email string
		if err := rows.Scan(&email); err != nil {
			return nil, err
		}
		result = append(result, email)
	}
	return result, rows.Err()
}
