package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakePinger struct {
	err error
}

func (p fakePinger) Ping(context.Context) error { return p.err }

func TestReady(t *testing.T) {
	health := NewHandler(fakePinger{}, "test")
	recorder := httptest.NewRecorder()
	health.Ready(recorder, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
}

func TestNotReadyDoesNotExposeDependencyError(t *testing.T) {
	health := NewHandler(fakePinger{err: errors.New("password=must-not-leak")}, "test")
	recorder := httptest.NewRecorder()
	health.Ready(recorder, httptest.NewRequest(http.MethodGet, "/ready", nil))
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", recorder.Code)
	}
	if body := recorder.Body.String(); body == "" || contains(body, "must-not-leak") {
		t.Fatalf("unsafe response body: %s", body)
	}
}

func contains(value, substring string) bool {
	for index := 0; index+len(substring) <= len(value); index++ {
		if value[index:index+len(substring)] == substring {
			return true
		}
	}
	return false
}
