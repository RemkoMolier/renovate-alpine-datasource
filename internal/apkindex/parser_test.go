package apkindex

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"strings"
	"testing"
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

func TestParseSimple(t *testing.T) {
	apkindex := strings.Join([]string{
		"C:QEl42+Jsiuy0sbECvL6bVvR6XHAA8=",
		"P:musl",
		"V:1.2.5-r8",
		"A:x86_64",
		"S:116684",
		"I:131072",
		"T:the musl c library (libc) implementation",
		"U:https://musl.libc.org/",
		"L:MIT",
		"o:musl",
		"m:Timo Teräs <timo.teras@iki.fi>",
		"t:1740691539",
		"c:7c1234567890abcdef1234567890abcdef12345678",
		"p:so:libc.musl-x86_64.so.1=1",
		"",
		"C:QEl42+Jsiuy0sbECvL6bVvR6XHAA9=",
		"P:alpine-baselayout",
		"V:3.6.8-r1",
		"A:x86_64",
		"S:3636",
		"I:4096",
		"T:Alpine base dir structure and init scripts",
		"U:https://gitlab.alpinelinux.org/alpine/aports",
		"L:GPL-2.0-only",
		"o:alpine-baselayout",
		"m:Natanael Copa <ncopa@alpinelinux.org>",
		"t:1740504474",
		"c:abcdef1234567890abcdef1234567890abcdef12",
		"",
		"C:QEl42+Jsiuy0sbECvL6bVvR6XHAA0=",
		"P:busybox",
		"V:1.37.0-r12",
		"A:x86_64",
		"S:973072",
		"I:217088",
		"T:Size optimized toolbox of many common UNIX utilities",
		"U:https://busybox.net/",
		"L:GPL-2.0-only",
		"o:busybox",
		"m:Sören Tempel <soeren+alpine@soeren-tempel.net>",
		"t:1740796909",
	}, "\n") + "\n"

	files := map[string]string{
		"APKINDEX":    apkindex,
		"DESCRIPTION": "abc123def4567890abc123def4567890abc123de\n",
	}

	data := buildTarGz(files)
	records, desc, err := Parse(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Parse: unexpected error: %v", err)
	}

	if want, got := "abc123def4567890abc123def4567890abc123de", desc; want != got {
		t.Errorf("DESCRIPTION commit id: want %q, got %q", want, got)
	}

	if want, got := 3, len(records); want != got {
		t.Fatalf("record count: want %d, got %d", want, got)
	}

	tests := []struct {
		idx     int
		name    string
		version string
		origin  string
		url     string
		buildTS int64
	}{
		{0, "musl", "1.2.5-r8", "musl", "https://musl.libc.org/", 1740691539},
		{1, "alpine-baselayout", "3.6.8-r1", "alpine-baselayout", "https://gitlab.alpinelinux.org/alpine/aports", 1740504474},
		{2, "busybox", "1.37.0-r12", "busybox", "https://busybox.net/", 1740796909},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := records[tt.idx]
			if r.Name != tt.name {
				t.Errorf("Name: want %q, got %q", tt.name, r.Name)
			}
			if r.Version != tt.version {
				t.Errorf("Version: want %q, got %q", tt.version, r.Version)
			}
			if r.Origin != tt.origin {
				t.Errorf("Origin: want %q, got %q", tt.origin, r.Origin)
			}
			if r.URL != tt.url {
				t.Errorf("URL: want %q, got %q", tt.url, r.URL)
			}
			if r.BuildTS != tt.buildTS {
				t.Errorf("BuildTS: want %d, got %d", tt.buildTS, r.BuildTS)
			}
		})
	}
}

func TestParseSubpackageFold(t *testing.T) {
	// foo, foo-dev, foo-doc: all share o:foo origin.
	apkindex := strings.Join([]string{
		"P:foo",
		"V:1.0-r0",
		"o:foo",
		"",
		"P:foo-dev",
		"V:1.0-r0",
		"o:foo",
		"",
		"P:foo-doc",
		"V:1.0-r0",
		"o:foo",
		"",
	}, "\n") + "\n"

	files := map[string]string{
		"APKINDEX":    apkindex,
		"DESCRIPTION": "abc\n",
	}

	data := buildTarGz(files)
	records, _, err := Parse(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Parse: unexpected error: %v", err)
	}

	if want, got := 3, len(records); want != got {
		t.Fatalf("record count: want %d, got %d", want, got)
	}

	for _, r := range records {
		if r.Origin != "foo" {
			t.Errorf("%s: Origin: want %q, got %q", r.Name, "foo", r.Origin)
		}
	}

	names := []string{"foo", "foo-dev", "foo-doc"}
	for i, want := range names {
		if records[i].Name != want {
			t.Errorf("record[%d]: Name: want %q, got %q", i, want, records[i].Name)
		}
	}
}

func TestParseOriginDefaultsToName(t *testing.T) {
	// No o: line — top-level package, origin should default to Name.
	apkindex := strings.Join([]string{
		"P:alpine-release",
		"V:3.22.0-r0",
		"",
	}, "\n") + "\n"

	files := map[string]string{
		"APKINDEX":    apkindex,
		"DESCRIPTION": "xyz\n",
	}

	data := buildTarGz(files)
	records, _, err := Parse(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Parse: unexpected error: %v", err)
	}

	if want, got := 1, len(records); want != got {
		t.Fatalf("record count: want %d, got %d", want, got)
	}

	r := records[0]
	if r.Name != "alpine-release" {
		t.Errorf("Name: want %q, got %q", "alpine-release", r.Name)
	}
	if r.Origin != "alpine-release" {
		t.Errorf("Origin: want %q (defaults to Name), got %q", "alpine-release", r.Origin)
	}
}

func TestParseMissingOptionalFields(t *testing.T) {
	// Missing U, t, and o lines.
	apkindex := strings.Join([]string{
		"P:minimal",
		"V:1.0.0",
		"",
	}, "\n") + "\n"

	files := map[string]string{
		"APKINDEX":    apkindex,
		"DESCRIPTION": "desc\n",
	}

	data := buildTarGz(files)
	records, _, err := Parse(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Parse: unexpected error: %v", err)
	}

	if want, got := 1, len(records); want != got {
		t.Fatalf("record count: want %d, got %d", want, got)
	}

	r := records[0]
	if r.URL != "" {
		t.Errorf("URL: want %q, got %q", "", r.URL)
	}
	if r.BuildTS != 0 {
		t.Errorf("BuildTS: want %d, got %d", 0, r.BuildTS)
	}
	if r.Origin != "minimal" {
		t.Errorf("Origin: want %q (defaults to Name), got %q", "minimal", r.Origin)
	}
}

func TestParseDescriptionOnly(t *testing.T) {
	// APKINDEX tar.gz without APKINDEX file (just DESCRIPTION).
	files := map[string]string{
		"DESCRIPTION": "deadbeef\n",
	}

	data := buildTarGz(files)
	records, desc, err := Parse(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Parse: unexpected error: %v", err)
	}

	if want, got := "deadbeef", desc; want != got {
		t.Errorf("DESCRIPTION: want %q, got %q", want, got)
	}
	if len(records) != 0 {
		t.Errorf("records: want 0, got %d", len(records))
	}
}

func TestParseEmptyArchive(t *testing.T) {
	data := buildTarGz(map[string]string{})
	records, desc, err := Parse(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Parse: unexpected error: %v", err)
	}
	if desc != "" {
		t.Errorf("DESCRIPTION: want %q, got %q", "", desc)
	}
	if len(records) != 0 {
		t.Errorf("records: want 0, got %d", len(records))
	}
}

func TestParseNoDescription(t *testing.T) {
	apkindex := strings.Join([]string{
		"P:foo",
		"V:1.0-r0",
		"",
	}, "\n") + "\n"

	files := map[string]string{
		"APKINDEX": apkindex,
	}

	data := buildTarGz(files)
	records, desc, err := Parse(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("Parse: unexpected error: %v", err)
	}

	if desc != "" {
		t.Errorf("DESCRIPTION: want %q, got %q", "", desc)
	}
	if want, got := 1, len(records); want != got {
		t.Errorf("records: want %d, got %d", want, got)
	}
}

func TestParseNonGzipInput(t *testing.T) {
	_, _, err := Parse(strings.NewReader("not gzip data"))
	if err == nil {
		t.Error("expected error for non-gzip input, got nil")
	}
}

// readAllGuard wraps an io.Reader and panics if a single Read call
// requests a buffer whose capacity exceeds maxBuf — this catches
// io.ReadAll-style bulk reads that grow a buffer to hold all data.
type readAllGuard struct {
	r      io.Reader
	maxBuf int
}

func (g *readAllGuard) Read(p []byte) (int, error) {
	if cap(p) > g.maxBuf {
		panic("readAllGuard: Read called with buffer capacity > maxBuf — bulk read detected")
	}
	return g.r.Read(p)
}

// TestParseStreamingNoReadAll verifies that Parse does not use io.ReadAll
// (or equivalent bulk-read strategies) internally. It wraps the input in a
// readAllGuard with a 4096-byte cap and uses a payload large enough that
// io.ReadAll would trip the guard during its buffer-growth loop.
func TestParseStreamingNoReadAll(t *testing.T) {
	// Build a payload large enough (~20 KB) to force io.ReadAll to grow
	// its internal buffer past the 4096-byte guard threshold.
	var records []string
	for i := range 450 {
		records = append(records,
			"P:pkg"+string(rune('a'+i%26))+string(rune('a'+(i/26)%26)),
			"V:1.0",
			"",
		)
	}
	apkindex := strings.Join(records, "\n") + "\n"

	data := buildTarGz(map[string]string{
		"APKINDEX":    apkindex,
		"DESCRIPTION": "commit\n",
	})

	// Wrap in guard: any Read with cap > 4096 panics.
	gr := &readAllGuard{r: bytes.NewReader(data), maxBuf: 4096}
	recordsOut, desc, err := Parse(gr)
	if err != nil {
		t.Fatalf("Parse: unexpected error: %v", err)
	}
	if len(recordsOut) != 450 {
		t.Fatalf("records: want 450, got %d", len(recordsOut))
	}
	if desc != "commit" {
		t.Errorf("DESCRIPTION: want %q, got %q", "commit", desc)
	}
}
