package checklisthttp

import (
	httprequest "github.com/Nidka/BPM-service/internal/core/httpserver/request"
	"github.com/Nidka/BPM-service/internal/core/httpserver/response"
	"github.com/Nidka/BPM-service/internal/features/checklists/domain"
	"github.com/Nidka/BPM-service/internal/shared/application/pagination"
	"github.com/go-chi/chi/v5"
	"net/http"
	"net/url"
	"time"
)

type runSummaryDTO struct {
	ID                string     `json:"id"`
	TemplateName      string     `json:"templateName"`
	LocationID        string     `json:"locationId"`
	LocationName      string     `json:"locationName"`
	PerformedBy       string     `json:"performedBy"`
	PerformedByName   string     `json:"performedByName"`
	Status            string     `json:"status"`
	CreatedAt         time.Time  `json:"createdAt"`
	SubmittedAt       *time.Time `json:"submittedAt,omitempty"`
	CompliancePercent *float64   `json:"compliancePercent,omitempty"`
	AnswerCount       int        `json:"answerCount"`
	QuestionCount     int        `json:"questionCount"`
}

func (h *Handler) history(w http.ResponseWriter, r *http.Request) {
	uid, ok := h.actor(w, r)
	if !ok {
		return
	}
	query, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(query["scope"]) > 1 || len(query["status"]) > 1 {
		h.fail(w, r, pagination.ErrInvalid)
		return
	}
	filter := domain.HistoryFilter{Scope: query.Get("scope"), Status: query.Get("status")}
	query.Del("scope")
	query.Del("status")
	page, err := httprequest.Pagination(query.Encode())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	values, err := h.service.History(r.Context(), uid, chi.URLParam(r, "organizationID"), page, filter)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	out := make([]runSummaryDTO, 0, len(values.Items))
	for _, v := range values.Items {
		out = append(out, runSummaryDTO{v.ID, v.TemplateName, v.LocationID, v.LocationName, v.PerformedBy, v.PerformedByName, v.Status, v.CreatedAt, v.SubmittedAt, v.CompliancePercent, v.AnswerCount, v.QuestionCount})
	}
	response.JSON(w, 200, map[string]any{"data": out, "pagination": map[string]string{"nextPageToken": values.NextToken}})
}
