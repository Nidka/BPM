package application

import (
	"context"
	inputformat "github.com/Nidka/BPM-service/internal/shared/domain/input"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/Nidka/BPM-service/internal/features/organizations/domain"
)

// CheckMember comprueba el alta sin escribirla y solo después de autorizar al gestor.
// AddMember vuelve a comprobar usuario, permisos y sedes al confirmar el alta.
func (s *Service) CheckMember(ctx context.Context, uid, org, email string) (result domain.MemberEligibility, err error) {
	if !inputformat.Email(email) {
		return "", &domain.ValidationError{Fields: map[string]string{"email": "Ingresa un correo válido."}}
	}
	email = strings.TrimSpace(email)
	err = s.withMemberManager(ctx, uid, org, func(db Store) error {
		var e error
		result, e = db.MemberEligibility(ctx, org, email)
		return e
	})
	return
}

// SearchMemberAccounts limita la búsqueda dirigida por correo al gestor autorizado.
// La ausencia de sugerencias no afirma que un correo incompleto esté sin registrar.
func (s *Service) SearchMemberAccounts(ctx context.Context, uid, org, query string) (result domain.MemberAccountMatches, err error) {
	query = strings.ToLower(strings.TrimSpace(query))
	if utf8.RuneCountInString(query) < 3 || len(query) > 320 || strings.IndexFunc(query, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }) >= 0 {
		return result, &domain.ValidationError{Fields: map[string]string{"query": "Escribe al menos 3 caracteres del correo, sin espacios."}}
	}
	result.Suggestions = []string{}
	err = s.withMemberManager(ctx, uid, org, func(db Store) error {
		var e error
		result.Suggestions, e = db.MemberAccountSuggestions(ctx, org, query)
		if e != nil {
			return e
		}
		if inputformat.Email(query) && strings.Contains(strings.SplitN(query, "@", 2)[1], ".") {
			status, e := db.MemberEligibility(ctx, org, query)
			if e != nil {
				return e
			}
			result.ExactStatus = &status
		}
		return nil
	})
	return
}

func (s *Service) withMemberManager(ctx context.Context, uid, org string, read func(Store) error) error {
	return s.work.Within(ctx, func(db Store) error {
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
		return read(db)
	})
}
