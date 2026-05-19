// Package store provides an in-memory package store with authoritative-per-repo
// merge semantics — when a (branch, repo) slice is replaced, all of that
// slice's contributions are dropped and rebuilt from scratch.
package store

import "time"

// Record is an orchestrator-enriched APKINDEX entry. The Name, Version, Origin,
// URL, and BuildTS fields mirror apkindex.Record (once #3 lands this will be
// imported from there). Repo is set by the orchestrator and tells Replace which
// repository the record came from; it is not part of the APKINDEX data model.
type Record struct {
	Name    string // APKINDEX package name (may differ from Origin for subpackages)
	Origin  string
	Version string
	URL     string
	BuildTS int64
	Repo    string
}

// Release is a single release in a PkgDoc. Branch is deliberately excluded from
// the JSON shape — the branch is implied by the request URL (§4.8, §5.3).
type Release struct {
	Version          string `json:"version"`
	Repo             string `json:"repo"`
	ReleaseTimestamp int64  `json:"releaseTimestamp"`
}

// PkgDoc is the branch-scoped document returned by Get. All releases that
// belong to the same origin on the same branch are folded together, even when
// they come from different repos (§4.4, subpackage fold).
type PkgDoc struct {
	Releases []Release `json:"releases"`
}

// ManifestPkg is one entry in the Manifest.Packages slice (§5.5).
type ManifestPkg struct {
	Name     string   `json:"name"`
	Versions []string `json:"versions"`
}

// Manifest is the top-level catalog (§5.5). It lists every branch, every
// package (origin), and every version known to the store.
type Manifest struct {
	Generated    string        `json:"generated"`
	Branches     []string      `json:"branches"`
	PackageCount int           `json:"packageCount"`
	Packages     []ManifestPkg `json:"packages"`
}

// RepoTag holds the conditional-request state for a single (branch, repo)
// combination (§9). Consumers use ETag / LastModified for HTTP conditional
// requests so they don't re-fetch unchanged APKINDEX data.
type RepoTag struct {
	ETag    string `json:"etag"`
	LastMod string `json:"lastModified"`
}

// Stats holds aggregate counts describing the store contents.
type Stats struct {
	Origins  int `json:"origins"`
	Repos    int `json:"repos"`
	Branches int `json:"branches"`
}

// Now returns the current UTC time. Package-level var so tests can override it.
var Now = func() time.Time { return time.Now().UTC() }
