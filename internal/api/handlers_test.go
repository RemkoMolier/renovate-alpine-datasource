package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/RemkoMolier/renovate-alpine-datasource/internal/store"
)

func TestPackageEndpointBranchScopedOnly(t *testing.T) {
	t.Parallel()

	s := store.New()
	s.Replace("v3.23", "main", []store.Record{{Origin: "foo", Version: "1.2.3-r0", BuildTS: 1700000000}})
	s.Replace("v3.22", "main", []store.Record{{Origin: "foo", Version: "1.2.2-r0", BuildTS: 1690000000}})

	a := New()
	a.RegisterDataHandlers(s)

	req := httptest.NewRequest(http.MethodGet, "/packages/v3.23/foo", nil)
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	if rec.Header().Get("Cache-Control") != packageCacheControl {
		t.Fatalf("Cache-Control = %q, want %q", rec.Header().Get("Cache-Control"), packageCacheControl)
	}

	var got store.PkgDoc
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	want := store.PkgDoc{Releases: []store.Release{{Version: "1.2.3-r0", Repo: "main", ReleaseTimestamp: 1700000000}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("PkgDoc = %#v, want %#v", got, want)
	}

	var generic map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &generic); err != nil {
		t.Fatalf("decode generic response: %v", err)
	}

	releases, ok := generic["releases"].([]any)
	if !ok || len(releases) != 1 {
		t.Fatalf("releases = %#v, want exactly one entry", generic["releases"])
	}

	entry, ok := releases[0].(map[string]any)
	if !ok {
		t.Fatalf("release entry type = %T, want object", releases[0])
	}

	if _, exists := entry["branch"]; exists {
		t.Fatalf("release entry unexpectedly contains branch field: %#v", entry)
	}

	if len(entry) != 3 {
		t.Fatalf("release entry field count = %d, want 3 (version, repo, releaseTimestamp)", len(entry))
	}
}

func TestPackageEndpointErrorsAndAlias(t *testing.T) {
	t.Parallel()

	s := store.New()
	s.Replace("v3.23", "main", []store.Record{{Origin: "foo", Version: "1.2.3-r0", BuildTS: 1700000000}})

	a := New()
	a.RegisterDataHandlers(s)

	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{name: "unknown package", path: "/packages/v3.23/notexist", wantStatus: http.StatusNotFound},
		{name: "unknown branch", path: "/packages/v9.99/foo", wantStatus: http.StatusNotFound},
		{name: "json alias", path: "/packages/v3.23/foo.json", wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			a.Handler().ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status code = %d, want %d", rec.Code, tt.wantStatus)
			}

			if rec.Header().Get("Cache-Control") != packageCacheControl {
				t.Fatalf("Cache-Control = %q, want %q", rec.Header().Get("Cache-Control"), packageCacheControl)
			}
		})
	}
}

func TestIndexEndpoint(t *testing.T) {
	t.Parallel()

	s := store.New()
	s.Replace("v3.23", "main", []store.Record{{Origin: "foo", Version: "1.2.3-r0", BuildTS: 1700000000}})

	a := New()
	a.RegisterDataHandlers(s)

	req := httptest.NewRequest(http.MethodGet, "/index.json", nil)
	rec := httptest.NewRecorder()
	a.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status code = %d, want %d", rec.Code, http.StatusOK)
	}

	var got store.Manifest
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.PackageCount != 1 {
		t.Fatalf("PackageCount = %d, want %d", got.PackageCount, 1)
	}

	if len(got.Branches) != 1 || got.Branches[0] != "v3.23" {
		t.Fatalf("Branches = %#v, want %#v", got.Branches, []string{"v3.23"})
	}

	if len(got.Packages) != 1 || got.Packages[0].Name != "foo" {
		t.Fatalf("Packages = %#v, want single package foo", got.Packages)
	}
}
