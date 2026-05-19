// Package apkindex implements a streaming parser for Alpine Linux
// APKINDEX.tar.gz files, extracting package metadata for downstream
// consumption by Renovate's customDatasources mechanism.
package apkindex

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Record holds the parsed fields from a single APKINDEX package entry.
type Record struct {
	Name    string // P: field
	Version string // V: field
	Origin  string // o: field; defaults to Name if absent
	URL     string // U: field
	BuildTS int64  // t: field (Unix seconds); 0 if missing
}

// Parse reads a .tar.gz APKINDEX archive from r and returns the parsed
// package records and the DESCRIPTION commit id. The reader is consumed
// as a stream — no io.ReadAll is used on the decompressed content.
func Parse(r io.Reader) (records []Record, commitDesc string, err error) {
	gzReader, err := gzip.NewReader(r)
	if err != nil {
		return nil, "", fmt.Errorf("apkindex: gzip reader: %w", err)
	}
	defer func() {
		if cerr := gzReader.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("apkindex: gzip close: %w", cerr)
		}
	}()

	tarReader := tar.NewReader(gzReader)

	for {
		hdr, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", fmt.Errorf("apkindex: tar reader: %w", err)
		}

		switch hdr.Name {
		case "APKINDEX":
			records, err = parseAPKINDEX(tarReader)
			if err != nil {
				return nil, "", fmt.Errorf("apkindex: parse APKINDEX: %w", err)
			}
		case "DESCRIPTION":
			commitDesc, err = readDescription(tarReader)
			if err != nil {
				return nil, "", fmt.Errorf("apkindex: read DESCRIPTION: %w", err)
			}
		}
	}

	return records, commitDesc, nil
}

// parseAPKINDEX reads APKINDEX records line-by-line from the tar entry
// body. Records are blocks of "K:value" lines separated by blank lines.
func parseAPKINDEX(r io.Reader) ([]Record, error) {
	var records []Record
	scanner := bufio.NewScanner(r)

	var cur *Record
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if cur != nil {
				if cur.Origin == "" {
					cur.Origin = cur.Name
				}
				records = append(records, *cur)
				cur = nil
			}
			continue
		}

		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}

		if cur == nil {
			cur = &Record{}
		}

		switch k {
		case "P":
			cur.Name = v
		case "V":
			cur.Version = v
		case "o":
			cur.Origin = v
		case "U":
			cur.URL = v
		case "t":
			ts, err := strconv.ParseInt(v, 10, 64)
			if err == nil {
				cur.BuildTS = ts
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Flush the last record if the file doesn't end with a blank line.
	if cur != nil {
		if cur.Origin == "" {
			cur.Origin = cur.Name
		}
		records = append(records, *cur)
	}

	return records, nil
}

// readDescription reads the DESCRIPTION file content as the commit id.
func readDescription(r io.Reader) (string, error) {
	scanner := bufio.NewScanner(r)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return strings.TrimSpace(strings.Join(lines, "\n")), nil
}
