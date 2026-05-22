package webSearch

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sync"
)

var zipHrefRe = regexp.MustCompile(`(?i)href=["']([^"']+\.zip[^"']*)["']`)

// FetchZipLinks fetches pageURL and returns all absolute URLs pointing to .zip files found in href attributes.
func FetchZipLinksInPage(pageURL string) ([]string, error) {
	resp, err := http.Get(pageURL) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", pageURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	base, err := url.Parse(pageURL)
	if err != nil {
		return nil, fmt.Errorf("parse base url: %w", err)
	}

	matches := zipHrefRe.FindAllSubmatch(body, -1)
	seen := make(map[string]struct{})
	var links []string

	for _, m := range matches {
		ref, err := url.Parse(string(m[1]))
		if err != nil {
			continue
		}
		abs := base.ResolveReference(ref).String()
		if _, ok := seen[abs]; !ok {
			seen[abs] = struct{}{}
			links = append(links, abs)
		}
	}

	return links, nil
}

func FetchZipLinksInPages(pageURLs []string) ([]string, error) {
	type result struct {
		links []string
		err   error
	}

	results := make([]result, len(pageURLs))
	var wg sync.WaitGroup
	wg.Add(len(pageURLs))

	for i, u := range pageURLs {
		go func(idx int, pageURL string) {
			defer wg.Done()
			links, err := FetchZipLinksInPage(pageURL)
			results[idx] = result{links: links, err: err}
		}(i, u)
	}

	wg.Wait()

	seen := make(map[string]struct{})
	var all []string
	var errs []error

	for _, r := range results {
		if r.err != nil {
			errs = append(errs, r.err)
			continue
		}
		for _, link := range r.links {
			if _, ok := seen[link]; !ok {
				seen[link] = struct{}{}
				all = append(all, link)
			}
		}
	}

	if len(errs) > 0 {
		return all, fmt.Errorf("%d page(s) failed: %v", len(errs), errs)
	}

	return all, nil
}
