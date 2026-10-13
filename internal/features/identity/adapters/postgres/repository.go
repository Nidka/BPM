package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/Nidka/BPM-service/internal/features/identity/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (repository *Repository) FindByUserID(ctx context.Context, userID string) (domain.Profile, error) {
	rows, err := repository.pool.Query(ctx, `
		SELECT
			u.id,
			u.email,
			u.display_name,
			COALESCE(u.phone_number, ''),
			u.platform_role,
			u.status,
			o.id,
			o.legal_name,
			o.trade_name,
			om.role,
			l.id::text,
			l.name,
			l.address,
			l.address_reference,
			l.district,
			l.province,
			l.department,
			l.timezone
		FROM application_users AS u
		LEFT JOIN organization_memberships AS om
			ON om.user_id = u.id
			AND om.status = 'ACTIVE'
		LEFT JOIN organizations AS o
			ON o.id = om.organization_id
			AND o.status = 'ACTIVE'
		LEFT JOIN membership_locations AS ml
			ON ml.organization_id = om.organization_id
			AND ml.user_id = om.user_id
		LEFT JOIN locations AS l
			ON l.organization_id = ml.organization_id
			AND l.id = ml.location_id
			AND l.status = 'ACTIVE'
		WHERE u.id = $1
		ORDER BY o.legal_name, o.id, l.name, l.id
	`, userID)
	if err != nil {
		return domain.Profile{}, fmt.Errorf("query current identity profile: %w", err)
	}
	defer rows.Close()

	var profile domain.Profile
	found := false
	organizationIndexes := make(map[string]int)
	for rows.Next() {
		var organizationID, legalName, tradeName, membershipRole sql.NullString
		var locationID, locationName, address, addressReference sql.NullString
		var district, province, department, timezone sql.NullString
		if err := rows.Scan(
			&profile.ID,
			&profile.Email,
			&profile.DisplayName,
			&profile.PhoneNumber,
			&profile.PlatformRole,
			&profile.Status,
			&organizationID,
			&legalName,
			&tradeName,
			&membershipRole,
			&locationID,
			&locationName,
			&address,
			&addressReference,
			&district,
			&province,
			&department,
			&timezone,
		); err != nil {
			return domain.Profile{}, fmt.Errorf("scan current identity profile: %w", err)
		}
		found = true

		if !organizationID.Valid {
			continue
		}
		index, exists := organizationIndexes[organizationID.String]
		if !exists {
			index = len(profile.Organizations)
			organizationIndexes[organizationID.String] = index
			profile.Organizations = append(profile.Organizations, domain.OrganizationMembership{
				Organization: domain.Organization{
					ID:        organizationID.String,
					LegalName: legalName.String,
					TradeName: tradeName.String,
				},
				Role:      domain.MembershipRole(membershipRole.String),
				Locations: make([]domain.Location, 0),
			})
		}

		if locationID.Valid {
			profile.Organizations[index].Locations = append(profile.Organizations[index].Locations, domain.Location{
				ID:               locationID.String,
				Name:             locationName.String,
				Address:          address.String,
				AddressReference: addressReference.String,
				District:         district.String,
				Province:         province.String,
				Department:       department.String,
				Timezone:         timezone.String,
			})
		}
	}
	if err := rows.Err(); err != nil {
		return domain.Profile{}, fmt.Errorf("iterate current identity profile: %w", err)
	}
	if !found {
		return domain.Profile{}, domain.ErrUserNotFound
	}
	if profile.Organizations == nil {
		profile.Organizations = make([]domain.OrganizationMembership, 0)
	}
	return profile, nil
}
