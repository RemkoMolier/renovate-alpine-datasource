package apkindex

import (
	"context"
	"fmt"
	"net/http"
)

// RepoTag holds the conditional-request state for a single (branch, repo)
// combination (§9). Consumers use ETag / LastModified for HTTP conditional
// requests and CommitDesc for DESCRIPTION short-circuiting so they don't
// re-merge unchanged APKINDEX data.
type RepoTag struct {
	ETag       string
	LastMod    string
	CommitDesc string
}

// Fetch performs an HTTP conditional GET for an APKINDEX.tar.gz at url.
//
// # Change detection (two-layer, §4.5)
//
// Layer 1 — HTTP conditional request. If prev carries an ETag or
// Last-Modified, they are sent as If-None-Match / If-Modified-Since.
// The server may respond 304 when nothing changed.
//
// Layer 2 — DESCRIPTION commit-id short-circuit. Even on a 200, a
// byte-identical APKINDEX may have new CDN headers. Parsing the
// DESCRIPTION lets us skip the merge when the commit id matches prev.
//
// On HTTP 304: returns (nil, prev, false, nil) — the response body is
// not read and Parse is not invoked.
//
// On HTTP 200 with the same DESCRIPTION commit id as prev.CommitDesc:
// returns (nil, newTag, false, nil) — the body is parsed to extract the
// commit id, but no records are returned since the merge is not needed.
//
// On HTTP 200 with a different DESCRIPTION commit id (or on the first
// fetch where prev.CommitDesc is empty): returns (records, newTag, true, nil).
func Fetch(ctx context.Context, client *http.Client, url string, prev RepoTag) ([]Record, RepoTag, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, RepoTag{}, false, fmt.Errorf("apkindex: create request: %w", err)
	}

	if prev.ETag != "" {
		req.Header.Set("If-None-Match", prev.ETag)
	}
	if prev.LastMod != "" {
		req.Header.Set("If-Modified-Since", prev.LastMod)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, RepoTag{}, false, fmt.Errorf("apkindex: request: %w", err)
	}
	defer func() {
		if cerr := resp.Body.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("apkindex: response body close: %w", cerr)
		}
	}()

	next := RepoTag{
		ETag:    resp.Header.Get("ETag"),
		LastMod: resp.Header.Get("Last-Modified"),
	}

	if resp.StatusCode == http.StatusNotModified {
		// Layer 1 hit: nothing changed upstream. Do not read the body.
		return nil, prev, false, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, RepoTag{}, false, fmt.Errorf("apkindex: unexpected status %d", resp.StatusCode)
	}

	records, desc, err := Parse(resp.Body)
	if err != nil {
		return nil, RepoTag{}, false, fmt.Errorf("apkindex: parse: %w", err)
	}

	next.CommitDesc = desc

	if prev.CommitDesc != "" && desc == prev.CommitDesc {
		// Layer 2 hit: new CDN headers, same content. Skip the merge.
		return nil, next, false, nil
	}

	return records, next, true, nil
}
