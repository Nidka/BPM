package application

import (
	"context"
	"errors"
	"github.com/Nidka/BPM-service/internal/features/organizations/domain"
	"testing"
)

func TestInvalidFormsNeverAccessDatabase(t *testing.T) {
	service := New(nil)
	_, err := service.CreateOrganization(context.Background(), "uid", domain.Organization{})
	var validation *domain.ValidationError
	if !errors.As(err, &validation) || validation.Fields["legalName"] == "" || validation.Fields["businessType"] == "" {
		t.Fatal("missing organization fields", err)
	}
	_, err = service.CreateLocation(context.Background(), "uid", "org", domain.Location{})
	if !errors.As(err, &validation) || validation.Fields["name"] == "" || validation.Fields["address"] == "" {
		t.Fatal("missing location fields", err)
	}
	err = service.AddMember(context.Background(), "uid", "org", "bad", "SUPER_ADMIN", nil)
	if !errors.As(err, &validation) || validation.Fields["email"] == "" || validation.Fields["role"] == "" {
		t.Fatal("missing member fields", err)
	}
	err = service.AddMember(context.Background(), "uid", "org", "test@example.invalid", "WORKER", nil)
	if !errors.As(err, &validation) || validation.Fields["locationIds"] == "" {
		t.Fatal("worker needs locations", err)
	}
}
