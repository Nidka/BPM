package application

import (
	"context"
	"errors"
	"github.com/Nidka/BPM-service/internal/core/contracts"
	"github.com/Nidka/BPM-service/internal/features/identity/domain"
	"github.com/Nidka/BPM-service/internal/shared/domain/input"
	"strings"
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
	if !input.Email(email) {
		return domain.Profile{}, domain.ErrUnauthenticated
	}
	name := strings.TrimSpace(displayName)
	if !input.PersonName(displayName, 2, 100) {
		return domain.Profile{}, &domain.ValidationError{Fields: map[string]string{"displayName": "Usa entre 2 y 100 caracteres: letras, espacios, apóstrofos o guiones; sin números."}}
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
