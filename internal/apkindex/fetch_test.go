package apkindex

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// countedBody wraps an io.ReadCloser and atomically counts Read calls.
type countedBody struct {
	io.ReadCloser
	reads *atomic.Int32
}

func (b *countedBody) Read(p []byte) (int, error) {
	b.reads.Add(1)
	return b.ReadCloser.Read(p)
}

// countingTransport wraps an http.RoundTripper and replaces every response
// body with a countedBody.
type countingTransport struct {
	inner http.RoundTripper
	reads atomic.Int32
}

func (t *countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.inner.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	resp.Body = &countedBody{ReadCloser: resp.Body, reads: &t.reads}
	return resp, nil
}

// newTestServer builds an httptest.Server and a client whose transport
// wraps the server's internal transport and counts reads on every
// response body.
func newTestServer(handler http.Handler) (*httptest.Server, *countingTransport, *http.Client) {
	srv := httptest.NewServer(handler)
	tr := &countingTransport{inner: srv.Client().Transport}
	client := &http.Client{Transport: tr}
	return srv, tr, client
}

// buildTestTarGz builds a .tar.gz fixture with an APKINDEX and DESCRIPTION.
func buildTestTarGz(commitDesc string, records []string) []byte {
	apkindex := strings.Join(records, "\n")
	if apkindex != "" && !strings.HasSuffix(apkindex, "\n") {
		apkindex += "\n"
	}
	files := map[string]string{}
	if apkindex != "" {
		files["APKINDEX"] = apkindex
	}
	if commitDesc != "" {
		files["DESCRIPTION"] = commitDesc + "\n"
	}
	return buildTarGz(files)
}

// newTestHandler returns an http.Handler that serves APKINDEX.tar.gz content,
// returning the given status code and headers.
func newTestHandler(status int, commitDesc string, records []string, etag, lastMod string) http.Handler {
	body := buildTestTarGz(commitDesc, records)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if etag != "" {
			w.Header().Set("ETag", etag)
		}
		if lastMod != "" {
			w.Header().Set("Last-Modified", lastMod)
		}
		if status != http.StatusNotModified {
			w.Header().Set("Content-Type", "application/x-gzip")
		}
		w.WriteHeader(status)
		if status != http.StatusNotModified {
			if _, err := w.Write(body); err != nil {
				panic("write body: " + err.Error())
			}
		}
	})
}

func TestFetchNotModified(t *testing.T) {
	// Server always returns 304 with an ETag. Fetch must not read the body.
	handler := newTestHandler(http.StatusNotModified, "abc123", nil, `"etag-1"`, "")
	srv, tr, client := newTestServer(handler)
	defer srv.Close()

	prev := RepoTag{ETag: `"etag-1"`, CommitDesc: "old"}

	records, next, changed, err := Fetch(context.Background(), client, srv.URL, prev)
	if err != nil {
		t.Fatalf("Fetch: unexpected error: %v", err)
	}
	if changed {
		t.Error("changed: want false, got true")
	}
	if records != nil {
		t.Errorf("records: want nil, got %d records", len(records))
	}
	if next.ETag != prev.ETag || next.CommitDesc != prev.CommitDesc {
		t.Errorf("next: want prev RepoTag %+v (body not read), got %+v", prev, next)
	}

	// Body must not have been read.
	if n := tr.reads.Load(); n != 0 {
		t.Errorf("body reads on 304: want 0, got %d", n)
	}
}

func TestFetchNotModifiedSendsIfModifiedSince(t *testing.T) {
	// Server expects If-Modified-Since and returns 304.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("If-Modified-Since"); got != "Wed, 21 Oct 2015 07:28:00 GMT" {
			t.Errorf("If-Modified-Since: want %q, got %q", "Wed, 21 Oct 2015 07:28:00 GMT", got)
		}
		w.WriteHeader(http.StatusNotModified)
	}))
	defer srv.Close()

	prev := RepoTag{LastMod: "Wed, 21 Oct 2015 07:28:00 GMT"}
	_, _, changed, err := Fetch(context.Background(), srv.Client(), srv.URL, prev)
	if err != nil {
		t.Fatalf("Fetch: unexpected error: %v", err)
	}
	if changed {
		t.Error("changed: want false, got true")
	}
}

func TestFetchNotModifiedDoesNotSendEmptyHeaders(t *testing.T) {
	// Empty prev must not send conditional headers.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") != "" {
			t.Error("If-None-Match: must be empty")
		}
		if r.Header.Get("If-Modified-Since") != "" {
			t.Error("If-Modified-Since: must be empty")
		}
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(buildTestTarGz("newcommit", []string{"P:pkg", "V:1.0", ""})); err != nil {
			panic("write body: " + err.Error())
		}
	}))
	defer srv.Close()

	_, _, _, err := Fetch(context.Background(), srv.Client(), srv.URL, RepoTag{})
	if err != nil {
		t.Fatalf("Fetch: unexpected error: %v", err)
	}
}

func TestFetchSameDescription(t *testing.T) {
	// Server returns 200 with a new ETag but the same DESCRIPTION commit id.
	handler := newTestHandler(http.StatusOK, "abc123",
		[]string{"P:foo", "V:1.0", ""}, `"etag-2"`, "")
	srv, tr, client := newTestServer(handler)
	defer srv.Close()

	prev := RepoTag{ETag: `"etag-1"`, CommitDesc: "abc123"}

	records, next, changed, err := Fetch(context.Background(), client, srv.URL, prev)
	if err != nil {
		t.Fatalf("Fetch: unexpected error: %v", err)
	}
	if changed {
		t.Error("changed: want false (same DESCRIPTION), got true")
	}
	if records != nil {
		t.Errorf("records: want nil, got %d records", len(records))
	}
	if next.ETag != `"etag-2"` {
		t.Errorf("next.ETag: want %q, got %q", `"etag-2"`, next.ETag)
	}
	if next.CommitDesc != "abc123" {
		t.Errorf("next.CommitDesc: want %q, got %q", "abc123", next.CommitDesc)
	}

	// Body was read (DESCRIPTION must be parsed to extract commit id).
	if n := tr.reads.Load(); n == 0 {
		t.Error("body reads on 200: want > 0 (must parse DESCRIPTION), got 0")
	}
}

func TestFetchDifferentDescription(t *testing.T) {
	// Server returns 200 with new ETag and different DESCRIPTION commit id.
	handler := newTestHandler(http.StatusOK, "def456",
		[]string{"P:foo", "V:2.0-r0", "o:foo", ""}, `"etag-2"`, "Thu, 01 Jan 2025 00:00:00 GMT")
	srv, _, client := newTestServer(handler)
	defer srv.Close()

	prev := RepoTag{ETag: `"etag-1"`, CommitDesc: "abc123"}

	records, next, changed, err := Fetch(context.Background(), client, srv.URL, prev)
	if err != nil {
		t.Fatalf("Fetch: unexpected error: %v", err)
	}
	if !changed {
		t.Error("changed: want true (different DESCRIPTION), got false")
	}
	if len(records) != 1 {
		t.Fatalf("records: want 1, got %d", len(records))
	}
	if records[0].Name != "foo" {
		t.Errorf("records[0].Name: want %q, got %q", "foo", records[0].Name)
	}
	if records[0].Version != "2.0-r0" {
		t.Errorf("records[0].Version: want %q, got %q", "2.0-r0", records[0].Version)
	}
	if next.ETag != `"etag-2"` {
		t.Errorf("next.ETag: want %q, got %q", `"etag-2"`, next.ETag)
	}
	if next.LastMod != "Thu, 01 Jan 2025 00:00:00 GMT" {
		t.Errorf("next.LastMod: want %q, got %q", "Thu, 01 Jan 2025 00:00:00 GMT", next.LastMod)
	}
	if next.CommitDesc != "def456" {
		t.Errorf("next.CommitDesc: want %q, got %q", "def456", next.CommitDesc)
	}
}

func TestFetchFirstRun(t *testing.T) {
	// First fetch (prev.CommitDesc empty) must always return changed=true.
	handler := newTestHandler(http.StatusOK, "abc123",
		[]string{"P:pkg", "V:1.0", ""}, "", "")
	srv, _, client := newTestServer(handler)
	defer srv.Close()

	records, next, changed, err := Fetch(context.Background(), client, srv.URL, RepoTag{})
	if err != nil {
		t.Fatalf("Fetch: unexpected error: %v", err)
	}
	if !changed {
		t.Error("changed: want true (first run), got false")
	}
	if len(records) != 1 {
		t.Fatalf("records: want 1, got %d", len(records))
	}
	if next.CommitDesc != "abc123" {
		t.Errorf("next.CommitDesc: want %q, got %q", "abc123", next.CommitDesc)
	}
}

func TestFetchNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, _, _, err := Fetch(context.Background(), srv.Client(), srv.URL, RepoTag{})
	if err == nil {
		t.Fatal("expected error for non-200/304 status, got nil")
	}
}

func TestFetchSubpackageFold(t *testing.T) {
	// Confirm records correctly fold subpackages when changed.
	handler := newTestHandler(http.StatusOK, "commit1",
		[]string{
			"P:foo", "V:1.0-r0", "o:foo", "",
			"P:foo-dev", "V:1.0-r0", "o:foo", "",
			"P:foo-doc", "V:1.0-r0", "o:foo", "",
		}, "", "")
	srv, _, client := newTestServer(handler)
	defer srv.Close()

	records, _, changed, err := Fetch(context.Background(), client, srv.URL, RepoTag{CommitDesc: "old"})
	if err != nil {
		t.Fatalf("Fetch: unexpected error: %v", err)
	}
	if !changed {
		t.Error("changed: want true, got false")
	}
	if len(records) != 3 {
		t.Fatalf("records: want 3, got %d", len(records))
	}
	for _, r := range records {
		if r.Origin != "foo" {
			t.Errorf("%s: Origin: want %q, got %q", r.Name, "foo", r.Origin)
		}
	}
}
