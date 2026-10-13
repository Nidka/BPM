package application

import (
	"context"
	"errors"
	"github.com/Nidka/BPM-service/internal/core/contracts"
	"github.com/Nidka/BPM-service/internal/features/identity/domain"
	"net/mail"
	"strings"
	"unicode"
	"unicode/utf8"
)

type RegistrationRepository interface {
	ProfileRepository
	CreateUser(context.Context, domain.NewUser) error
}
type Register struct{ repository RegistrationRepository }

func NewRegister(repository RegistrationRepository) *Register {
	return &Register{repository: repository}
}

func (uc *Register) Execute(ctx context.Context, identity contracts.Identity, displayName string) (domain.Profile, error) {
	if strings.TrimSpace(identity.UID) == "" {
		return domain.Profile{}, domain.ErrUnauthenticated
	}
	if !identity.EmailVerified {
		return domain.Profile{}, domain.ErrEmailUnverified
	}
	email := strings.TrimSpace(identity.Email)
	parsed, err := mail.ParseAddress(email)
	if err != nil || parsed.Address != email || len(email) > 320 {
		return domain.Profile{}, domain.ErrUnauthenticated
	}
	name := strings.TrimSpace(displayName)
	if utf8.RuneCountInString(name) < 2 || utf8.RuneCountInString(name) > 100 || strings.IndexFunc(name, unicode.IsControl) >= 0 {
		return domain.Profile{}, &domain.ValidationError{Fields: map[string]string{"displayName": "Ingresa un nombre de entre 2 y 100 caracteres, sin caracteres de control."}}
	}
	profile, err := uc.repository.FindByUserID(ctx, identity.UID)
	if err == nil {
		return activeProfile(profile)
	}
	if !errors.Is(err, domain.ErrUserNotFound) {
		return domain.Profile{}, err
	}
	// Alta mínima; los reintentos nunca cambian estado, rol ni permisos existentes.
	if err := uc.repository.CreateUser(ctx, domain.NewUser{ID: identity.UID, Email: email, DisplayName: name}); err != nil {
		return domain.Profile{}, err
	}
	profile, err = uc.repository.FindByUserID(ctx, identity.UID)
	if err != nil {
		return domain.Profile{}, err
	}
	return activeProfile(profile)
}

func activeProfile(profile domain.Profile) (domain.Profile, error) {
	if !profile.IsActive() {
		return domain.Profile{}, domain.ErrUserInactive
	}
	if profile.Organizations == nil {
		profile.Organizations = []domain.OrganizationMembership{}
	}
	return profile, nil
}
