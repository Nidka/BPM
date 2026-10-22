package application

import (
	"context"
	"errors"
	"github.com/Nidka/BPM-service/internal/core/contracts"
	"github.com/Nidka/BPM-service/internal/features/identity/domain"
	"testing"
)

type registrationStub struct {
	user      domain.NewUser
	profile   domain.Profile
	exists    bool
	createErr error
	creates   int
}

func (r *registrationStub) FindByUserID(context.Context, string) (domain.Profile, error) {
	if !r.exists {
		return domain.Profile{}, domain.ErrUserNotFound
	}
	return r.profile, nil
}
func (r *registrationStub) CreateUser(_ context.Context, user domain.NewUser) error {
	r.creates++
	r.user = user
	if r.createErr != nil {
		return r.createErr
	}
	r.exists = true
	r.profile = domain.Profile{ID: user.ID, Email: user.Email, DisplayName: user.DisplayName,
		PlatformRole: domain.PlatformRoleUser, Status: domain.UserStatusActive}
	return nil
}
func TestRegisterUsesVerifiedIdentityAndIsIdempotent(t *testing.T) {
	repo := &registrationStub{}
	uc := NewRegister(repo)
	identity := contracts.Identity{UID: "uid", Email: "person@example.com", EmailVerified: true}
	profile, err := uc.Execute(context.Background(), identity, "  Persona BPM  ")
	if err != nil || profile.PlatformRole != domain.PlatformRoleUser || len(profile.Organizations) != 0 || repo.user.ID != "uid" || repo.user.Email != identity.Email || repo.user.DisplayName != "Persona BPM" {
		t.Fatalf("invalid registration: %+v err=%v", profile, err)
	}
	_, err = uc.Execute(context.Background(), identity, "Changed Name")
	if err != nil || repo.creates != 1 || repo.profile.DisplayName != "Persona BPM" {
		t.Fatal("retry modified existing profile")
	}
}
func TestRegisterRejectsUnverifiedOrInvalidFieldsBeforeWriting(t *testing.T) {
	cases := []struct {
		identity contracts.Identity
		name     string
		want     error
	}{
		{contracts.Identity{}, "Person", domain.ErrUnauthenticated},
		{contracts.Identity{UID: "uid", Email: "x@example.com"}, "Person", domain.ErrEmailUnverified},
		{contracts.Identity{UID: "uid", Email: "bad", EmailVerified: true}, "Person", domain.ErrUnauthenticated},
	}
	for _, tc := range cases {
		repo := &registrationStub{}
		_, err := NewRegister(repo).Execute(context.Background(), tc.identity, tc.name)
		if !errors.Is(err, tc.want) || repo.creates != 0 {
			t.Fatalf("expected safe rejection, got %v", err)
		}
	}
	for _, name := range []string{"", "a", "Bad\nName", "Ana2", "Persona 123", "Ana 🙂"} {
		repo := &registrationStub{}
		_, err := NewRegister(repo).Execute(context.Background(), contracts.Identity{UID: "uid", Email: "x@example.com", EmailVerified: true}, name)
		var validation *domain.ValidationError
		if !errors.As(err, &validation) || validation.Fields["displayName"] == "" || repo.creates != 0 {
			t.Fatal("missing field validation")
		}
	}
}
func TestRegisterDoesNotReactivateInactiveUser(t *testing.T) {
	repo := &registrationStub{exists: true, profile: domain.Profile{Status: domain.UserStatusInactive}}
	_, err := NewRegister(repo).Execute(context.Background(), contracts.Identity{UID: "uid", Email: "x@example.com", EmailVerified: true}, "Person")
	if !errors.Is(err, domain.ErrUserInactive) || repo.creates != 0 {
		t.Fatal("must not reactivate user")
	}
}
func TestRegisterPropagatesConflictWithoutMergingUid(t *testing.T) {
	repo := &registrationStub{createErr: domain.ErrRegistrationConflict}
	_, err := NewRegister(repo).Execute(context.Background(), contracts.Identity{UID: "uid", Email: "x@example.com", EmailVerified: true}, "Person")
	if !errors.Is(err, domain.ErrRegistrationConflict) || repo.exists {
		t.Fatal("must not merge users")
	}
}
