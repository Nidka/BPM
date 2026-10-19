package application

import (
	"context"
	"net/mail"
	"strings"

	"github.com/Nidka/BPM-service/internal/features/organizations/domain"
)

// CheckMember comprueba el alta sin escribirla y solo después de autorizar al gestor.
// AddMember vuelve a comprobar usuario, permisos y sedes al confirmar el alta.
func (s *Service) CheckMember(ctx context.Context, uid, org, email string) (result domain.MemberEligibility, err error) {
	email = strings.TrimSpace(email)
	address, e := mail.ParseAddress(email)
	if e != nil || address.Address != email || len(email) > 320 {
		return "", &domain.ValidationError{Fields: map[string]string{"email": "Ingresa un correo válido."}}
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
		if !domain.CanManage(role) {
			return domain.ErrDenied
		}
		result, e = db.MemberEligibility(ctx, org, email)
		return e
	})
	return
}
