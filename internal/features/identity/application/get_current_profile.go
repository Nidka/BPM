package application

import (
	"context"
	"strings"

	"github.com/Nidka/BPM/internal/core/contracts"
	"github.com/Nidka/BPM/internal/features/identity/domain"
)

type GetCurrentProfile struct {
	repository ProfileRepository
}

func NewGetCurrentProfile(repository ProfileRepository) *GetCurrentProfile {
	return &GetCurrentProfile{repository: repository}
}

func (useCase *GetCurrentProfile) Execute(ctx context.Context, identity contracts.Identity) (domain.Profile, error) {
	if strings.TrimSpace(identity.UID) == "" {
		return domain.Profile{}, domain.ErrUnauthenticated
	}

	profile, err := useCase.repository.FindByUserID(ctx, identity.UID)
	if err != nil {
		return domain.Profile{}, err
	}
	if !profile.IsActive() {
		return domain.Profile{}, domain.ErrUserInactive
	}
	if profile.Organizations == nil {
		profile.Organizations = make([]domain.OrganizationMembership, 0)
	}
	return profile, nil
}
