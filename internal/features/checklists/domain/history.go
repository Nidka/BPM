package domain

import "time"

// RunSummary identifica una revisión concreta sin descargar todas sus respuestas.
type RunSummary struct {
	ID, TemplateName, LocationID, LocationName, PerformedBy, PerformedByName, Status string
	CreatedAt                                                                        time.Time
	SubmittedAt                                                                      *time.Time
	CompliancePercent                                                                *float64
	AnswerCount, QuestionCount                                                       int
}

type HistoryFilter struct{ Scope, Status string }

func CanPerformRuns(role string) bool { return role == "CHEF" }
func CanSuperviseRuns(role string) bool {
	return role == "ADMIN" || role == "CHEF" || role == "SUPER_ADMIN"
}
