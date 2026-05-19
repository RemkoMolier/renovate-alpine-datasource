package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/RemkoMolier/renovate-alpine-datasource/internal/store"
)

const packageCacheControl = "max-age=300"

type packageStore interface {
	Get(branch, name string) (store.PkgDoc, bool)
	Manifest() store.Manifest
}

// RegisterDataHandlers registers HTTP handlers that expose Renovate datasource JSON endpoints.
func (a *API) RegisterDataHandlers(s packageStore) {
	a.mux.HandleFunc("GET /packages/{branch}/{name}", a.handlePackage(s))
	a.mux.HandleFunc("GET /index.json", a.handleIndex(s))
}

func (a *API) handlePackage(s packageStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		branch := r.PathValue("branch")
		name := strings.TrimSuffix(r.PathValue("name"), ".json")
		a.servePackageDoc(w, r, branch, name, s)
	}
}

func (a *API) servePackageDoc(w http.ResponseWriter, r *http.Request, branch, name string, s packageStore) {
	w.Header().Set("Cache-Control", packageCacheControl)

	doc, ok := s.Get(branch, name)
	if !ok {
		http.NotFound(w, r)

		return
	}

	writeJSON(w, doc)
}

func (a *API) handleIndex(s packageStore) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, s.Manifest())
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, "encode response", http.StatusInternalServerError)
	}
}
