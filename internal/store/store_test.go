package store

import (
	"encoding/json"
	"sync"
	"testing"
)

func makeRecords(repo string, entries ...struct {
	name, origin, version string
	ts                    int64
}) []Record {
	recs := make([]Record, len(entries))
	for i, e := range entries {
		recs[i] = Record{
			Name:    e.name,
			Origin:  e.origin,
			Version: e.version,
			Repo:    repo,
			BuildTS: e.ts,
		}
	}
	return recs
}

func TestNewStoreIsNotReady(t *testing.T) {
	s := New()
	if s.Ready() {
		t.Error("new store must not be ready")
	}
}

func TestMarkReady(t *testing.T) {
	s := New()
	s.MarkReady()
	if !s.Ready() {
		t.Error("store must be ready after MarkReady")
	}
}

func TestGetEmptyStore(t *testing.T) {
	s := New()
	_, ok := s.Get("v3.23", "nonexistent")
	if ok {
		t.Error("Get on empty store must return false")
	}
}

func TestStatsEmptyStore(t *testing.T) {
	s := New()
	st := s.Stats()
	if st.Origins != 0 || st.Repos != 0 || st.Branches != 0 {
		t.Errorf("empty store stats must all be zero, got %+v", st)
	}
}

func TestManifestEmptyStore(t *testing.T) {
	s := New()
	m := s.Manifest()
	if m.PackageCount != 0 {
		t.Errorf("empty manifest PackageCount must be 0, got %d", m.PackageCount)
	}
	if len(m.Branches) != 0 || len(m.Packages) != 0 {
		t.Errorf("empty manifest must have no branches or packages")
	}
}

func TestReplaceAndGet(t *testing.T) {
	s := New()
	recs := makeRecords("main",
		struct {
			name, origin, version string
			ts                    int64
		}{"foo", "foo", "1.0.0", 100},
		struct {
			name, origin, version string
			ts                    int64
		}{"foo", "foo", "1.1.0", 200},
	)
	s.Replace("v3.23", "main", recs)

	doc, ok := s.Get("v3.23", "foo")
	if !ok {
		t.Fatal("Get must return true for inserted package")
	}
	if len(doc.Releases) != 2 {
		t.Fatalf("want 2 releases, got %d", len(doc.Releases))
	}
}

func TestGetWrongBranch(t *testing.T) {
	s := New()
	recs := makeRecords("main",
		struct {
			name, origin, version string
			ts                    int64
		}{"foo", "foo", "1.0.0", 100},
	)
	s.Replace("v3.23", "main", recs)

	_, ok := s.Get("v3.24", "foo")
	if ok {
		t.Error("Get must return false for wrong branch")
	}
}

func TestGetWrongOrigin(t *testing.T) {
	s := New()
	recs := makeRecords("main",
		struct {
			name, origin, version string
			ts                    int64
		}{"foo", "foo", "1.0.0", 100},
	)
	s.Replace("v3.23", "main", recs)

	_, ok := s.Get("v3.23", "bar")
	if ok {
		t.Error("Get must return false for wrong origin")
	}
}

func TestRevocation(t *testing.T) {
	// The critical §F8 test: a version present in round 1 but absent in
	// round 2 must disappear. A third round on a different branch must not
	// disturb the first branch's data.
	s := New()

	// Round 1.
	s.Replace("v3.23", "main", makeRecords("main",
		struct {
			name, origin, version string
			ts                    int64
		}{"foo", "foo", "v1", 100},
		struct {
			name, origin, version string
			ts                    int64
		}{"foo", "foo", "v2", 200},
	))

	doc, ok := s.Get("v3.23", "foo")
	if !ok {
		t.Fatal("round 1: Get must return true")
	}
	if len(doc.Releases) != 2 {
		t.Fatalf("round 1: want 2 releases, got %d", len(doc.Releases))
	}

	// Round 2: v1 is revoked — only v2 remains.
	s.Replace("v3.23", "main", makeRecords("main",
		struct {
			name, origin, version string
			ts                    int64
		}{"foo", "foo", "v2", 200},
	))

	doc, ok = s.Get("v3.23", "foo")
	if !ok {
		t.Fatal("round 2: Get must return true")
	}
	if len(doc.Releases) != 1 {
		t.Fatalf("round 2: want 1 release after revocation, got %d", len(doc.Releases))
	}
	if doc.Releases[0].Version != "v2" {
		t.Errorf("round 2: remaining release must be v2, got %s", doc.Releases[0].Version)
	}

	// Round 3: add releases on a different branch — v3.23 data must be intact.
	s.Replace("v3.24", "main", makeRecords("main",
		struct {
			name, origin, version string
			ts                    int64
		}{"foo", "foo", "v3", 300},
	))

	doc, ok = s.Get("v3.23", "foo")
	if !ok {
		t.Fatal("round 3: v3.23 data must still exist")
	}
	if len(doc.Releases) != 1 || doc.Releases[0].Version != "v2" {
		t.Errorf("round 3: v3.23 data must be unchanged, got %+v", doc.Releases)
	}

	doc, ok = s.Get("v3.24", "foo")
	if !ok {
		t.Fatal("round 3: v3.24 data must exist")
	}
	if len(doc.Releases) != 1 || doc.Releases[0].Version != "v3" {
		t.Errorf("round 3: v3.24 must have v3, got %+v", doc.Releases)
	}
}

func TestSubpackageFold(t *testing.T) {
	// §4.4: packages sharing the same origin must be folded under that origin.
	s := New()
	recs := []Record{
		{Name: "foo", Origin: "foo", Version: "1.0", Repo: "main", BuildTS: 100},
		{Name: "foo-dev", Origin: "foo", Version: "1.0", Repo: "main", BuildTS: 100},
	}
	s.Replace("v3.23", "main", recs)

	doc, ok := s.Get("v3.23", "foo")
	if !ok {
		t.Fatal("Get must return true for origin foo")
	}
	if len(doc.Releases) != 2 {
		t.Fatalf("subpackage fold: want 2 releases for origin foo, got %d", len(doc.Releases))
	}

	// foo-dev should NOT be retrievable under its own name.
	_, ok = s.Get("v3.23", "foo-dev")
	if ok {
		t.Error("subpackage fold: foo-dev must not be a separate origin")
	}
}

func TestMultiRepoFold(t *testing.T) {
	// Releases from different repos on the same branch fold together.
	s := New()
	s.Replace("v3.23", "main", makeRecords("main",
		struct {
			name, origin, version string
			ts                    int64
		}{"foo", "foo", "v1", 100},
	))
	s.Replace("v3.23", "community", makeRecords("community",
		struct {
			name, origin, version string
			ts                    int64
		}{"foo", "foo", "v2", 200},
	))

	doc, ok := s.Get("v3.23", "foo")
	if !ok {
		t.Fatal("Get must return true")
	}
	if len(doc.Releases) != 2 {
		t.Fatalf("multi-repo fold: want 2 releases, got %d", len(doc.Releases))
	}
}

func TestEmptyReplaceClears(t *testing.T) {
	// Replacing with empty records must clear all releases for that (branch, repo).
	s := New()
	s.Replace("v3.23", "main", makeRecords("main",
		struct {
			name, origin, version string
			ts                    int64
		}{"foo", "foo", "v1", 100},
	))
	s.Replace("v3.23", "main", nil)

	_, ok := s.Get("v3.23", "foo")
	if ok {
		t.Error("empty Replace must clear all releases")
	}
}

func TestReplaceIsAtomic(t *testing.T) {
	// A partial Replace must not leave the store in an inconsistent state.
	// Here we verify that replacing a repo only affects that repo's data.
	s := New()
	s.Replace("v3.23", "main", makeRecords("main",
		struct {
			name, origin, version string
			ts                    int64
		}{"foo", "foo", "v1", 100},
	))
	s.Replace("v3.23", "community", makeRecords("community",
		struct {
			name, origin, version string
			ts                    int64
		}{"foo", "foo", "v2", 200},
	))

	// Replace only main — community data must survive.
	s.Replace("v3.23", "main", makeRecords("main",
		struct {
			name, origin, version string
			ts                    int64
		}{"foo", "foo", "v3", 300},
	))

	doc, _ := s.Get("v3.23", "foo")
	if len(doc.Releases) != 2 {
		t.Fatalf("want 2 releases (1 main + 1 community), got %d", len(doc.Releases))
	}

	has := func(version string) bool {
		for _, r := range doc.Releases {
			if r.Version == version {
				return true
			}
		}
		return false
	}
	if !has("v3") {
		t.Error("main release v3 must be present")
	}
	if !has("v2") {
		t.Error("community release v2 must survive main-only Replace")
	}
	if has("v1") {
		t.Error("v1 must be gone (replaced)")
	}
}

func TestStats(t *testing.T) {
	s := New()
	s.Replace("v3.23", "main", makeRecords("main",
		struct {
			name, origin, version string
			ts                    int64
		}{"foo", "foo", "v1", 100},
		struct {
			name, origin, version string
			ts                    int64
		}{"bar", "bar", "v1", 100},
	))
	s.Replace("v3.24", "community", makeRecords("community",
		struct {
			name, origin, version string
			ts                    int64
		}{"foo", "foo", "v2", 200},
	))

	st := s.Stats()
	if st.Origins != 2 {
		t.Errorf("Origins: want 2, got %d", st.Origins)
	}
	if st.Repos != 2 {
		t.Errorf("Repos: want 2, got %d", st.Repos)
	}
	if st.Branches != 2 {
		t.Errorf("Branches: want 2, got %d", st.Branches)
	}
}

func TestManifest(t *testing.T) {
	s := New()
	s.Replace("v3.24", "main", makeRecords("main",
		struct {
			name, origin, version string
			ts                    int64
		}{"foo", "foo", "v1.0", 100},
		struct {
			name, origin, version string
			ts                    int64
		}{"foo", "foo", "v1.1", 200},
	))
	s.Replace("v3.23", "main", makeRecords("main",
		struct {
			name, origin, version string
			ts                    int64
		}{"bar", "bar", "v2.0", 300},
	))

	m := s.Manifest()
	if m.PackageCount != 2 {
		t.Errorf("PackageCount: want 2, got %d", m.PackageCount)
	}
	if len(m.Branches) != 2 {
		t.Errorf("Branches: want 2, got %d", len(m.Branches))
	}
	if len(m.Packages) != 2 {
		t.Errorf("Packages: want 2, got %d", len(m.Packages))
	}
	if m.Generated == "" {
		t.Error("Generated must not be empty")
	}

	// Branches must be sorted.
	for i := 1; i < len(m.Branches); i++ {
		if m.Branches[i] < m.Branches[i-1] {
			t.Errorf("Branches not sorted: %v", m.Branches)
		}
	}
	// Packages must be sorted by name.
	for i := 1; i < len(m.Packages); i++ {
		if m.Packages[i].Name < m.Packages[i-1].Name {
			t.Errorf("Packages not sorted by name: %v", m.Packages)
		}
	}
}

func TestRepoTag(t *testing.T) {
	s := New()

	_, ok := s.RepoTag("v3.23", "main")
	if ok {
		t.Error("RepoTag on empty store must return false")
	}

	tag := RepoTag{ETag: `"abc123"`, LastMod: "Mon, 01 Jan 2024 00:00:00 GMT"}
	s.SetRepoTag("v3.23", "main", tag)

	got, ok := s.RepoTag("v3.23", "main")
	if !ok {
		t.Fatal("RepoTag must return true after SetRepoTag")
	}
	if got.ETag != tag.ETag || got.LastMod != tag.LastMod {
		t.Errorf("RepoTag round-trip: want %+v, got %+v", tag, got)
	}

	// Different branch must not collide.
	_, ok = s.RepoTag("v3.24", "main")
	if ok {
		t.Error("RepoTag on different branch must return false")
	}
}

func TestReleaseJSONShape(t *testing.T) {
	// §4.8: Release JSON must not include a Branch field.
	r := Release{
		Version:          "1.0.0",
		Repo:             "main",
		ReleaseTimestamp: 1234567890,
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if _, exists := m["branch"]; exists {
		t.Error("Release JSON must not contain a 'branch' key")
	}
	if _, exists := m["Branch"]; exists {
		t.Error("Release JSON must not contain a 'Branch' key")
	}
	wantKeys := map[string]bool{"version": true, "repo": true, "releaseTimestamp": true}
	for k := range m {
		if !wantKeys[k] {
			t.Errorf("Release JSON has unexpected key %q", k)
		}
	}
}

func TestRaceGetDuringReplace(t *testing.T) {
	// Parallel Get calls during a Replace must not panic and must always see a
	// consistent snapshot (§N7).
	s := New()

	// Seed some data.
	recs := make([]Record, 50)
	for i := range recs {
		recs[i] = Record{
			Name:    "pkg",
			Origin:  "pkg",
			Version: "v" + string(rune('0'+i%10)),
			Repo:    "main",
			BuildTS: int64(i),
		}
	}
	s.Replace("edge", "main", recs)

	var wg sync.WaitGroup

	// Writer: replaces repeatedly.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range 100 {
			ri := make([]Record, i%10+1)
			for j := range ri {
				ri[j] = Record{
					Name:    "pkg",
					Origin:  "pkg",
					Version: "v" + string(rune('0'+(i+j)%10)),
					Repo:    "main",
					BuildTS: int64(i*100 + j),
				}
			}
			s.Replace("edge", "main", ri)
		}
	}()

	// Readers: Get concurrently.
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				doc, ok := s.Get("edge", "pkg")
				if ok {
					// Just access the slice — must not panic.
					_ = len(doc.Releases)
				}
				// Also exercise Stats, Manifest, RepoTag, Ready.
				_ = s.Stats()
				_ = s.Manifest()
				s.RepoTag("edge", "main")
				_ = s.Ready()
			}
		}()
	}

	wg.Wait()
}
