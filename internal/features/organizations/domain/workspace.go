package domain

import "errors"

var (
	ErrDenied            = errors.New("workspace access denied")
	ErrConflict          = errors.New("resource already exists")
	ErrMemberUnavailable = errors.New("member unavailable")
)

type ValidationError struct{ Fields map[string]string }

func (e *ValidationError) Error() string { return "invalid workspace input" }

type Actor struct{ ID, Role string }
type Organization struct{ ID, LegalName, TradeName, TaxID, BusinessType, ContactName, ContactPhone, ContactEmail string }
type Location struct{ ID, Name, Address, District, ContactPhone, Timezone string }
type Member struct {
	ID, Name, Email, Role string
	LocationIDs           []string
}
type Entry struct{ ID, Title, Detail, Status string }
type Workspace struct {
	Organization                   Organization
	Role                           string
	Locations                      []Location
	Members                        []Member
	Courses, Checklists, Incidents []Entry
}

func CanManage(role string) bool { return role == "SUPER_ADMIN" || role == "ADMIN" }
