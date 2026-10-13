package identityhttp

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Nidka/BPM-service/internal/core/contracts"
	"github.com/Nidka/BPM-service/internal/core/httpserver/middleware"
	"github.com/Nidka/BPM-service/internal/core/httpserver/response"
	"github.com/Nidka/BPM-service/internal/features/identity/domain"
)

type CurrentProfileGetter interface {
	Execute(ctx context.Context, identity contracts.Identity) (domain.Profile, error)
}

type Handler struct {
	getCurrentProfile CurrentProfileGetter
	logger            *slog.Logger
}

func NewHandler(getCurrentProfile CurrentProfileGetter, logger *slog.Logger) *Handler {
	return &Handler{getCurrentProfile: getCurrentProfile, logger: logger}
}

func (handler *Handler) Me(writer http.ResponseWriter, request *http.Request) {
	requestID := middleware.RequestIDFromContext(request.Context())
	identity, ok := middleware.IdentityFromContext(request.Context())
	if !ok {
		response.Unauthenticated(writer, requestID)
		return
	}

	profile, err := handler.getCurrentProfile.Execute(request.Context(), identity)
	if err != nil {
		handler.writeError(writer, request, err)
		return
	}
	response.JSON(writer, http.StatusOK, map[string]any{"data": newProfileResponse(profile)})
}

func (handler *Handler) writeError(writer http.ResponseWriter, request *http.Request, err error) {
	requestID := middleware.RequestIDFromContext(request.Context())
	switch {
	case errors.Is(err, domain.ErrUnauthenticated):
		response.Unauthenticated(writer, requestID)
	case errors.Is(err, domain.ErrUserNotFound):
		response.Error(writer, http.StatusForbidden, requestID, response.APIError{
			Code:    "USER_NOT_REGISTERED",
			Message: "Tu cuenta no está habilitada en la plataforma.",
		})
	case errors.Is(err, domain.ErrUserInactive):
		response.Error(writer, http.StatusForbidden, requestID, response.APIError{
			Code:    "USER_INACTIVE",
			Message: "Tu cuenta se encuentra inactiva.",
		})
	default:
		handler.logger.Error("identity request failed",
			"component", "identity",
			"request_id", requestID,
			"error_type", "internal",
		)
		response.InternalError(writer, requestID)
	}
}

type profileResponse struct {
	ID            string                 `json:"id"`
	Email         string                 `json:"email"`
	DisplayName   string                 `json:"displayName"`
	PhoneNumber   string                 `json:"phoneNumber,omitempty"`
	PlatformRole  domain.PlatformRole    `json:"platformRole"`
	Organizations []organizationResponse `json:"organizations"`
}

type organizationResponse struct {
	ID        string                `json:"id"`
	LegalName string                `json:"legalName"`
	TradeName string                `json:"tradeName,omitempty"`
	Role      domain.MembershipRole `json:"role"`
	Locations []locationResponse    `json:"locations"`
}

type locationResponse struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Address          string `json:"address"`
	AddressReference string `json:"addressReference,omitempty"`
	District         string `json:"district,omitempty"`
	Province         string `json:"province,omitempty"`
	Department       string `json:"department,omitempty"`
	Timezone         string `json:"timezone"`
}

func newProfileResponse(profile domain.Profile) profileResponse {
	result := profileResponse{
		ID:            profile.ID,
		Email:         profile.Email,
		DisplayName:   profile.DisplayName,
		PhoneNumber:   profile.PhoneNumber,
		PlatformRole:  profile.PlatformRole,
		Organizations: make([]organizationResponse, 0, len(profile.Organizations)),
	}
	for _, membership := range profile.Organizations {
		organization := organizationResponse{
			ID:        membership.Organization.ID,
			LegalName: membership.Organization.LegalName,
			TradeName: membership.Organization.TradeName,
			Role:      membership.Role,
			Locations: make([]locationResponse, 0, len(membership.Locations)),
		}
		for _, location := range membership.Locations {
			organization.Locations = append(organization.Locations, locationResponse{
				ID:               location.ID,
				Name:             location.Name,
				Address:          location.Address,
				AddressReference: location.AddressReference,
				District:         location.District,
				Province:         location.Province,
				Department:       location.Department,
				Timezone:         location.Timezone,
			})
		}
		result.Organizations = append(result.Organizations, organization)
	}
	return result
}
