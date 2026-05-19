package store

import (
	"sort"
	"strings"
	"sync"
)

// branchRepoKey returns a composite key for a (branch, repo) pair.
func branchRepoKey(branch, repo string) string {
	return branch + ":" + repo
}

// Store is an in-memory package store with authoritative-per-repo merge
// semantics (§4.4). Reads are protected by a shared lock; writes are fully
// atomic under an exclusive lock (§N7).
type Store struct {
	mu    sync.RWMutex
	ready bool

	// data[origin][branchRepoKey] = releases contributed by that (branch, repo).
	data map[string]map[string][]Release

	// repoTags[branchRepoKey] = conditional-request state (§9).
	repoTags map[string]RepoTag
}

// New returns an empty Store.
func New() *Store {
	return &Store{}
}

// Ready reports whether the store has been marked ready.
func (s *Store) Ready() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.ready
}

// MarkReady marks the store as ready. Once marked, the store cannot be unmarked.
func (s *Store) MarkReady() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ready = true
}

// Replace drops the (branch, repo) slice for every affected origin and rebuilds
// it exclusively from records. Callers must pass every record for a given
// (branch, repo) on each call — this is what makes revoked versions disappear
// (requirement §F8). The operation is atomic with respect to readers.
func (s *Store) Replace(branch, repo string, records []Record) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := branchRepoKey(branch, repo)

	// Collect origins affected by this replacement: those in the incoming
	// records plus any origin that previously had entries for this (branch, repo).
	affected := make(map[string]struct{})
	for _, r := range records {
		affected[r.Origin] = struct{}{}
	}
	for origin, branches := range s.data {
		if _, ok := branches[key]; ok {
			affected[origin] = struct{}{}
		}
	}

	// Drop the (branch, repo) slice for every affected origin.
	for origin := range affected {
		delete(s.data[origin], key)
		if len(s.data[origin]) == 0 {
			delete(s.data, origin)
		}
	}

	// Rebuild from the incoming records.
	for _, r := range records {
		if s.data == nil {
			s.data = make(map[string]map[string][]Release)
		}
		if s.data[r.Origin] == nil {
			s.data[r.Origin] = make(map[string][]Release)
		}
		rel := Release{
			Version:          r.Version,
			Repo:             r.Repo,
			ReleaseTimestamp: r.ReleaseTimestamp,
		}
		s.data[r.Origin][key] = append(s.data[r.Origin][key], rel)
	}
}

// Get returns the branch-scoped PkgDoc for the given origin. The returned
// releases come from every repo on that branch (§4.8 — branch is the
// discriminator). ok is false when no releases exist for (branch, origin).
func (s *Store) Get(branch, name string) (PkgDoc, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	origin, ok := s.data[name]
	if !ok {
		return PkgDoc{}, false
	}

	prefix := branch + ":"
	var doc PkgDoc
	for key, releases := range origin {
		if strings.HasPrefix(key, prefix) {
			doc.Releases = append(doc.Releases, releases...)
		}
	}
	if len(doc.Releases) == 0 {
		return PkgDoc{}, false
	}
	return doc, true
}

// Manifest returns the top-level catalog (§5.5).
func (s *Store) Manifest() Manifest {
	s.mu.RLock()
	defer s.mu.RUnlock()

	branches := make(map[string]struct{})
	pkgVersions := make(map[string]map[string]struct{}) // origin -> set of versions

	for origin, originData := range s.data {
		for key, releases := range originData {
			br, _, _ := strings.Cut(key, ":")
			branches[br] = struct{}{}
			if pkgVersions[origin] == nil {
				pkgVersions[origin] = make(map[string]struct{})
			}
			for _, r := range releases {
				pkgVersions[origin][r.Version] = struct{}{}
			}
		}
	}

	m := Manifest{
		Generated:    Now().Format("2006-01-02T15:04:05Z"),
		PackageCount: len(pkgVersions),
	}

	for b := range branches {
		m.Branches = append(m.Branches, b)
	}
	sort.Strings(m.Branches)

	for name, vset := range pkgVersions {
		mp := ManifestPkg{Name: name}
		for v := range vset {
			mp.Versions = append(mp.Versions, v)
		}
		sort.Strings(mp.Versions)
		m.Packages = append(m.Packages, mp)
	}
	sort.Slice(m.Packages, func(i, j int) bool {
		return m.Packages[i].Name < m.Packages[j].Name
	})

	return m
}

// Stats returns aggregate counts of the store contents.
func (s *Store) Stats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	origins := make(map[string]struct{})
	repos := make(map[string]struct{})
	branches := make(map[string]struct{})

	for origin := range s.data {
		origins[origin] = struct{}{}
		for key := range s.data[origin] {
			br, rp, _ := strings.Cut(key, ":")
			branches[br] = struct{}{}
			repos[rp] = struct{}{}
		}
	}

	return Stats{
		Origins:  len(origins),
		Repos:    len(repos),
		Branches: len(branches),
	}
}

// RepoTag returns the conditional-request tag for (branch, repo), if any.
func (s *Store) RepoTag(branch, repo string) (RepoTag, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.repoTags == nil {
		return RepoTag{}, false
	}
	tag, ok := s.repoTags[branchRepoKey(branch, repo)]
	return tag, ok
}

// SetRepoTag stores the conditional-request tag for (branch, repo).
func (s *Store) SetRepoTag(branch, repo string, tag RepoTag) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.repoTags == nil {
		s.repoTags = make(map[string]RepoTag)
	}
	s.repoTags[branchRepoKey(branch, repo)] = tag
}
