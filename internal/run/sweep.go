// Package run implements the sweep orchestrator that drives APKINDEX
// fetching, parsing, and store population in a single-shot cycle.
package run

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/RemkoMolier/renovate-alpine-datasource/internal/apkindex"
	"github.com/RemkoMolier/renovate-alpine-datasource/internal/store"
)

// SweepOpts carries the parameters for one sweep cycle.
type SweepOpts struct {
	Branches   []string
	Repos      []string
	Arch       string
	Mirror     string
	HTTPClient *http.Client
}

// Sweep runs one sweep cycle: for each (branch, repo) pair it fetches
// APKINDEX.tar.gz, parses records, and merges into the store. 404 responses
// are skipped silently. Other HTTP or parse errors are logged but not fatal
// — one failure does not abort the cycle. If at least one (branch, repo)
// succeeded the store is marked ready. Returns an aggregated error so the
// caller can see every failure.
func Sweep(ctx context.Context, s *store.Store, opts SweepOpts) error {
	client := opts.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	var errs []error
	anySuccess := false

	for _, branch := range opts.Branches {
		for _, repo := range opts.Repos {
			url := fmt.Sprintf("%s/%s/%s/%s/APKINDEX.tar.gz", opts.Mirror, branch, repo, opts.Arch)

			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				errs = append(errs, fmt.Errorf("sweep %s/%s: new request: %w", branch, repo, err))
				continue
			}

			resp, err := client.Do(req)
			if err != nil {
				slog.Error("sweep request", "branch", branch, "repo", repo, "err", err)
				errs = append(errs, fmt.Errorf("sweep %s/%s: %w", branch, repo, err))
				continue
			}

			if resp.StatusCode == http.StatusNotFound {
				closeResp(resp)
				continue
			}

			if resp.StatusCode != http.StatusOK {
				closeResp(resp)
				slog.Error("sweep non-OK status", "branch", branch, "repo", repo, "status", resp.StatusCode)
				errs = append(errs, fmt.Errorf("sweep %s/%s: HTTP %d", branch, repo, resp.StatusCode))
				continue
			}

			records, _, err := apkindex.Parse(resp.Body)
			closeResp(resp)
			if err != nil {
				slog.Error("sweep parse", "branch", branch, "repo", repo, "err", err)
				errs = append(errs, fmt.Errorf("sweep %s/%s: parse: %w", branch, repo, err))
				continue
			}

			storeRecords := make([]store.Record, len(records))
			for i, r := range records {
				storeRecords[i] = store.Record{
					Name:    r.Name,
					Origin:  r.Origin,
					Version: r.Version,
					URL:     r.URL,
					BuildTS: r.BuildTS,
					Repo:    repo,
				}
			}

			s.Replace(branch, repo, storeRecords)
			anySuccess = true
		}
	}

	if anySuccess {
		s.MarkReady()
	}

	return errors.Join(errs...)
}

// closeResp drains and closes the response body, logging close errors.
func closeResp(resp *http.Response) {
	if err := resp.Body.Close(); err != nil {
		slog.Warn("sweep close response body", "err", err)
	}
}
