package application

import (
	"context"
	"errors"
	"testing"

	"github.com/Nidka/BPM-service/internal/core/contracts"
	"github.com/Nidka/BPM-service/internal/features/identity/domain"
)

type profileRepositoryStub struct {
	profile domain.Profile
	err     error
	userID  string
}

func (repository *profileRepositoryStub) FindByUserID(_ context.Context, userID string) (domain.Profile, error) {
	repository.userID = userID
	return repository.profile, repository.err
}

func TestGetCurrentProfileReturnsActiveUser(t *testing.T) {
	repository := &profileRepositoryStub{profile: domain.Profile{
		ID:     "firebase-uid",
		Status: domain.UserStatusActive,
	}}
	useCase := NewGetCurrentProfile(repository)

	profile, err := useCase.Execute(context.Background(), contracts.Identity{UID: "firebase-uid"})
	if err != nil {
		t.Fatalf("execute use case: %v", err)
	}
	if profile.ID != "firebase-uid" || repository.userID != "firebase-uid" {
		t.Fatalf("unexpected profile: %+v", profile)
	}
	if profile.Organizations == nil {
		t.Fatal("organizations must be an empty array, not null")
	}
}

func TestGetCurrentProfileRejectsMissingIdentity(t *testing.T) {
	useCase := NewGetCurrentProfile(&profileRepositoryStub{})

	_, err := useCase.Execute(context.Background(), contracts.Identity{})
	if !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatalf("expected unauthenticated error, got %v", err)
	}
}

func TestGetCurrentProfileRejectsInactiveUser(t *testing.T) {
	useCase := NewGetCurrentProfile(&profileRepositoryStub{profile: domain.Profile{
		ID:     "firebase-uid",
		Status: domain.UserStatusInactive,
	}})

	_, err := useCase.Execute(context.Background(), contracts.Identity{UID: "firebase-uid"})
	if !errors.Is(err, domain.ErrUserInactive) {
		t.Fatalf("expected inactive user error, got %v", err)
	}
}
