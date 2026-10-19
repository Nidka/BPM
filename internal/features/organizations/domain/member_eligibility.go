package domain

// MemberEligibility no revela perfiles, credenciales ni membresías de otras empresas.
type MemberEligibility string

const (
	MemberReady           MemberEligibility = "READY"
	MemberAlreadyAssigned MemberEligibility = "ALREADY_MEMBER"
	MemberUnavailable     MemberEligibility = "UNAVAILABLE"
)
