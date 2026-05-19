package run

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/RemkoMolier/renovate-alpine-datasource/internal/store"
)

// buildTarGz creates an in-memory .tar.gz with the given files (name → content).
func buildTarGz(files map[string]string) []byte {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for name, content := range files {
		hdr := &tar.Header{
			Name: name,
			Size: int64(len(content)),
			Mode: 0o644,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			panic("write tar header: " + err.Error())
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			panic("write tar body: " + err.Error())
		}
	}
	if err := tw.Close(); err != nil {
		panic("close tar: " + err.Error())
	}
	if err := gw.Close(); err != nil {
		panic("close gzip: " + err.Error())
	}
	return buf.Bytes()
}

// makeAPKINDEX creates APKINDEX content with packages named after the
// (branch, repo) pair so tests can verify which data landed in the store.
func makeAPKINDEX(branch, repo string) string {
	pkgBase := fmt.Sprintf("%s_%s", strings.ReplaceAll(branch, ".", "_"), repo)
	return strings.Join([]string{
		fmt.Sprintf("P:%s_pkg1", pkgBase),
		"V:1.0.0",
		fmt.Sprintf("o:%s_pkg1", pkgBase),
		"U:https://example.com",
		"t:1700000000",
		"",
		fmt.Sprintf("P:%s_pkg2", pkgBase),
		"V:2.0.0",
		fmt.Sprintf("o:%s_pkg2", pkgBase),
		"U:https://example.com",
		"t:1700000001",
		"",
	}, "\n") + "\n"
}

func TestSweepHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// URL pattern: /{branch}/{repo}/{arch}/APKINDEX.tar.gz
		parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/"), "/", 4)
		if len(parts) != 4 {
			http.NotFound(w, r)
			return
		}
		branch, repo := parts[0], parts[1]
		body := buildTarGz(map[string]string{
			"APKINDEX":    makeAPKINDEX(branch, repo),
			"DESCRIPTION": fmt.Sprintf("commit-%s-%s\n", branch, repo),
		})
		w.Header().Set("Content-Type", "application/octet-stream")
		if _, err := w.Write(body); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer srv.Close()

	st := store.New()
	opts := SweepOpts{
		Branches:   []string{"v3.22", "v3.23"},
		Repos:      []string{"main", "community"},
		Arch:       "x86_64",
		Mirror:     srv.URL,
		HTTPClient: srv.Client(),
	}

	err := Sweep(context.Background(), st, opts)
	if err != nil {
		t.Fatalf("Sweep: unexpected error: %v", err)
	}
	if !st.Ready() {
		t.Error("store should be ready after successful sweep")
	}

	stats := st.Stats()
	if stats.Origins == 0 {
		t.Error("store should be populated")
	}

	// Verify data exists for each (branch, repo) pair.
	for _, branch := range opts.Branches {
		for _, repo := range opts.Repos {
			pkgBase := fmt.Sprintf("%s_%s_pkg1", strings.ReplaceAll(branch, ".", "_"), repo)
			doc, ok := st.Get(branch, pkgBase)
			if !ok {
				t.Errorf("expected origin %q on branch %q to exist", pkgBase, branch)
				continue
			}
			if len(doc.Releases) != 1 {
				t.Errorf("expected 1 release for %s/%s (origin=%q), got %d", branch, repo, pkgBase, len(doc.Releases))
				continue
			}
			if doc.Releases[0].Repo != repo {
				t.Errorf("expected repo %q, got %q", repo, doc.Releases[0].Repo)
			}
		}
	}
}

func TestSweepSkip404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/"), "/", 4)
		if len(parts) != 4 {
			http.NotFound(w, r)
			return
		}
		branch, repo := parts[0], parts[1]

		// testing branch returns 404
		if branch == "testing" {
			http.NotFound(w, r)
			return
		}

		body := buildTarGz(map[string]string{
			"APKINDEX":    makeAPKINDEX(branch, repo),
			"DESCRIPTION": fmt.Sprintf("commit-%s-%s\n", branch, repo),
		})
		w.Header().Set("Content-Type", "application/octet-stream")
		if _, err := w.Write(body); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer srv.Close()

	st := store.New()
	opts := SweepOpts{
		Branches:   []string{"v3.22", "testing"},
		Repos:      []string{"main"},
		Arch:       "x86_64",
		Mirror:     srv.URL,
		HTTPClient: srv.Client(),
	}

	err := Sweep(context.Background(), st, opts)
	if err != nil {
		t.Fatalf("Sweep: unexpected error: %v", err)
	}
	if !st.Ready() {
		t.Error("store should be ready — v3.22 succeeded")
	}

	// v3.22 should be present.
	doc, ok := st.Get("v3.22", "v3_22_main_pkg1")
	if !ok {
		t.Error("expected v3.22 data to be present")
	}
	if len(doc.Releases) != 1 {
		t.Errorf("expected 1 release, got %d", len(doc.Releases))
	}

	// testing should not be present.
	_, ok = st.Get("testing", "testing_main_pkg1")
	if ok {
		t.Error("testing branch should not have data (404)")
	}
}

func TestSweep500AggregatedError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/"), "/", 4)
		if len(parts) != 4 {
			http.NotFound(w, r)
			return
		}
		branch, repo := parts[0], parts[1]

		if branch == "v3.22" && repo == "community" {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		body := buildTarGz(map[string]string{
			"APKINDEX":    makeAPKINDEX(branch, repo),
			"DESCRIPTION": fmt.Sprintf("commit-%s-%s\n", branch, repo),
		})
		w.Header().Set("Content-Type", "application/octet-stream")
		if _, err := w.Write(body); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer srv.Close()

	st := store.New()
	opts := SweepOpts{
		Branches:   []string{"v3.22", "v3.23"},
		Repos:      []string{"main", "community"},
		Arch:       "x86_64",
		Mirror:     srv.URL,
		HTTPClient: srv.Client(),
	}

	err := Sweep(context.Background(), st, opts)
	if err == nil {
		t.Fatal("expected aggregated error, got nil")
	}

	// Verify the community repo on v3.22 is absent.
	_, ok := st.Get("v3.22", "v3_22_community_pkg1")
	if ok {
		t.Error("v3.22/community should be absent after 500 error")
	}

	// Other repos should still be present.
	doc, ok := st.Get("v3.22", "v3_22_main_pkg1")
	if !ok {
		t.Error("v3.22/main should be present")
	}
	if len(doc.Releases) != 1 {
		t.Errorf("expected 1 release, got %d", len(doc.Releases))
	}

	doc, ok = st.Get("v3.23", "v3_23_community_pkg1")
	if !ok {
		t.Error("v3.23/community should be present")
	}
	if len(doc.Releases) != 1 {
		t.Errorf("expected 1 release, got %d", len(doc.Releases))
	}

	if !st.Ready() {
		t.Error("store should be ready — other (branch, repo) pairs succeeded")
	}
}

func TestSweepNoSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	st := store.New()
	opts := SweepOpts{
		Branches:   []string{"v3.22"},
		Repos:      []string{"main"},
		Arch:       "x86_64",
		Mirror:     srv.URL,
		HTTPClient: srv.Client(),
	}

	err := Sweep(context.Background(), st, opts)
	if err == nil {
		t.Fatal("expected error when nothing succeeds")
	}
	if st.Ready() {
		t.Error("store should NOT be ready when nothing succeeded")
	}
}

func TestSweepOptsDefaultClient(t *testing.T) {
	// When HTTPClient is nil, Sweep should use http.DefaultClient.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := buildTarGz(map[string]string{
			"APKINDEX":    makeAPKINDEX("edge", "main"),
			"DESCRIPTION": "commit\n",
		})
		w.Header().Set("Content-Type", "application/octet-stream")
		if _, err := w.Write(body); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer srv.Close()

	st := store.New()
	opts := SweepOpts{
		Branches: []string{"edge"},
		Repos:    []string{"main"},
		Arch:     "x86_64",
		Mirror:   srv.URL,
		// HTTPClient intentionally nil
	}

	err := Sweep(context.Background(), st, opts)
	if err != nil {
		t.Fatalf("Sweep: unexpected error: %v", err)
	}
	if !st.Ready() {
		t.Error("store should be ready")
	}
}

func TestSweepErrorsJoin(t *testing.T) {
	// All repos fail → errors.Join returns non-nil with all errors.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	st := store.New()
	opts := SweepOpts{
		Branches:   []string{"v3.22", "v3.23"},
		Repos:      []string{"main", "community"},
		Arch:       "x86_64",
		Mirror:     srv.URL,
		HTTPClient: srv.Client(),
	}

	err := Sweep(context.Background(), st, opts)
	if err == nil {
		t.Fatal("expected error")
	}

	// errors.Join returns an error wrapping each individual error.
	errs := err.(interface{ Unwrap() []error }).Unwrap()
	if len(errs) != 4 {
		t.Errorf("expected 4 errors (2 branches × 2 repos), got %d", len(errs))
	}

	// Verify errors.Is works on each.
	for _, e := range errs {
		if !errors.Is(err, e) {
			t.Errorf("errors.Is(err, e) should be true for %v", e)
		}
	}
}

// errTransport is an http.RoundTripper that always returns an error.
type errTransport struct{ err error }

func (t *errTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return nil, t.err
}

func TestSweepTransportError(t *testing.T) {
	// When the HTTP transport fails, the error accumulates but doesn't abort.
	st := store.New()
	opts := SweepOpts{
		Branches: []string{"v3.22"},
		Repos:    []string{"main"},
		Arch:     "x86_64",
		Mirror:   "http://unused.example",
		HTTPClient: &http.Client{
			Transport: &errTransport{err: errors.New("connection refused")},
		},
	}

	err := Sweep(context.Background(), st, opts)
	if err == nil {
		t.Fatal("expected error from transport failure")
	}
	if st.Ready() {
		t.Error("store should not be ready when nothing succeeded")
	}
}

func TestSweepParseError(t *testing.T) {
	// Return 200 OK but with invalid gzip data — parse should fail, error
	// accumulates, but doesn't abort the sweep.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		if _, err := w.Write([]byte("not a valid gzip file")); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer srv.Close()

	st := store.New()
	opts := SweepOpts{
		Branches:   []string{"v3.22", "v3.23"},
		Repos:      []string{"main"},
		Arch:       "x86_64",
		Mirror:     srv.URL,
		HTTPClient: srv.Client(),
	}

	err := Sweep(context.Background(), st, opts)
	if err == nil {
		t.Fatal("expected aggregated error from parse failure")
	}
	if st.Ready() {
		t.Error("store should not be ready when nothing parsed successfully")
	}
}
