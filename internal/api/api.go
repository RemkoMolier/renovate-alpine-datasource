package api

import (
	"net/http"
	"sync"
)

// API exposes HTTP endpoints for process liveness and readiness probes.
type API struct {
	mu    sync.RWMutex
	ready bool
	mux   *http.ServeMux
}

func New() *API {
	a := &API{mux: http.NewServeMux()}
	a.mux.HandleFunc("GET /healthz", a.handleHealthz)
	a.mux.HandleFunc("GET /readyz", a.handleReadyz)

	return a
}

func (a *API) Handler() http.Handler {
	return a.mux
}

// SetReady updates readiness state for /readyz.
//
// The orchestrator calls this once the first APKINDEX sweep succeeds.
func (a *API) SetReady(ready bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.ready = ready
}

func (a *API) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (a *API) handleReadyz(w http.ResponseWriter, _ *http.Request) {
	if a.isReady() {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))

		return
	}

	w.WriteHeader(http.StatusServiceUnavailable)
	_, _ = w.Write([]byte("not ready"))
}

func (a *API) isReady() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	return a.ready
}
