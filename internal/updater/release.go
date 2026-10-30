package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// DefaultAPIURL is the GitHub API endpoint for Eirin's latest release.
const DefaultAPIURL = "https://api.github.com/repos/hawawa4/eirin/releases/latest"

// checksumsAsset is the release asset listing the SHA-256 of every other asset.
const checksumsAsset = "SHA256SUMS"

// Asset is one downloadable file attached to a release.
type Asset struct {
	Name string
	URL  string
	Size int64
}

// Release is a published GitHub release.
type Release struct {
	// Version is the tag without any leading "v": "1.6.0".
	Version string
	Notes   string
	PageURL string
	Assets  []Asset
}

// Client talks to GitHub. The zero value uses the real Eirin repository.
type Client struct {
	HTTP   *http.Client
	APIURL string
}

func (c Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return http.DefaultClient
}

func (c Client) apiURL() string {
	if c.APIURL != "" {
		return c.APIURL
	}
	return DefaultAPIURL
}

func (c Client) get(ctx context.Context, url, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "eirin-updater")
	req.Header.Set("Accept", accept)
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return resp, nil
}

// Latest fetches the newest published release (GitHub skips drafts and
// pre-releases for this endpoint).
func (c Client) Latest(ctx context.Context) (Release, error) {
	resp, err := c.get(ctx, c.apiURL(), "application/vnd.github+json")
	if err != nil {
		return Release{}, fmt.Errorf("checking for updates: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var raw struct {
		TagName string `json:"tag_name"`
		Body    string `json:"body"`
		HTMLURL string `json:"html_url"`
		Assets  []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
			Size int64  `json:"size"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&raw); err != nil {
		return Release{}, fmt.Errorf("reading release info: %w", err)
	}
	rel := Release{
		Version: strings.TrimPrefix(raw.TagName, "v"),
		Notes:   raw.Body,
		PageURL: raw.HTMLURL,
	}
	if !IsRelease(rel.Version) {
		return Release{}, fmt.Errorf("latest release has an unrecognised tag %q", raw.TagName)
	}
	for _, a := range raw.Assets {
		rel.Assets = append(rel.Assets, Asset{Name: a.Name, URL: a.URL, Size: a.Size})
	}
	return rel, nil
}

// asset returns the release asset called name.
func (r Release) asset(name string) (Asset, bool) {
	for _, a := range r.Assets {
		if a.Name == name {
			return a, true
		}
	}
	return Asset{}, false
}

// checksumFor downloads the release's SHA256SUMS and returns the hex digest
// listed for the asset called name.
func (c Client) checksumFor(ctx context.Context, rel Release, name string) (string, error) {
	sums, ok := rel.asset(checksumsAsset)
	if !ok {
		return "", errors.New("this release has no " + checksumsAsset + " file, so the download can't be verified")
	}
	resp, err := c.get(ctx, sums.URL, "application/octet-stream")
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	return parseChecksum(string(data), name)
}

// parseChecksum finds name in `sha256sum` output ("<hex>  <name>", with an
// optional "*" binary marker before the name).
func parseChecksum(sums, name string) (string, error) {
	for _, line := range strings.Split(sums, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || strings.TrimPrefix(fields[1], "*") != name {
			continue
		}
		if len(fields[0]) != 64 {
			return "", fmt.Errorf("malformed checksum for %s", name)
		}
		return strings.ToLower(fields[0]), nil
	}
	return "", fmt.Errorf("no checksum listed for %s", name)
}
