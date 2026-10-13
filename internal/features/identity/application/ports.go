package application

import (
	"context"

	"github.com/Nidka/BPM/internal/features/identity/domain"
)

type ProfileRepository interface {
	FindByUserID(ctx context.Context, userID string) (domain.Profile, error)
}
