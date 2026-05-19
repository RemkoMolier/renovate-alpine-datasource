package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAPIProbes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "healthz returns ok",
			path:       "/healthz",
			wantStatus: http.StatusOK,
			wantBody:   "ok",
		},
		{
			name:       "readyz returns not ready initially",
			path:       "/readyz",
			wantStatus: http.StatusServiceUnavailable,
			wantBody:   "not ready",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			a := New()
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()

			a.Handler().ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status code = %d, want %d", rec.Code, tt.wantStatus)
			}

			if rec.Body.String() != tt.wantBody {
				t.Fatalf("body = %q, want %q", rec.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestAPISetReady(t *testing.T) {
	t.Parallel()

	a := New()
	a.SetReady(true)

	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	if rec.Body.String() != "ready" {
		t.Fatalf("body = %q, want %q", rec.Body.String(), "ready")
	}
}
