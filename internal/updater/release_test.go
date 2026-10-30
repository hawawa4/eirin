package updater

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLatest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("GitHub rejects requests without a User-Agent")
		}
		_, _ = w.Write([]byte(`{"tag_name":"1.7.0","body":"notes","html_url":"https://example/r",
			"assets":[{"name":"eirin-linux-amd64.deb","browser_download_url":"https://example/d","size":42}]}`))
	}))
	defer srv.Close()

	rel, err := Client{APIURL: srv.URL}.Latest(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if rel.Version != "1.7.0" || rel.Notes != "notes" || rel.PageURL != "https://example/r" {
		t.Errorf("unexpected release: %+v", rel)
	}
	if a, ok := rel.asset("eirin-linux-amd64.deb"); !ok || a.Size != 42 {
		t.Errorf("asset lookup failed: %+v %v", a, ok)
	}
}

func TestLatestErrors(t *testing.T) {
	notFound := httptest.NewServer(http.NotFoundHandler())
	defer notFound.Close()
	if _, err := (Client{APIURL: notFound.URL}).Latest(context.Background()); err == nil {
		t.Error("expected an error on 404")
	}

	badTag := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"tag_name":"nightly"}`))
	}))
	defer badTag.Close()
	if _, err := (Client{APIURL: badTag.URL}).Latest(context.Background()); err == nil {
		t.Error("expected an error on a non-version tag")
	}
}

func TestParseChecksum(t *testing.T) {
	hash := strings.Repeat("ab", 32)
	sums := hash + "  eirin-linux-amd64.zip\n" + strings.Repeat("cd", 32) + " *eirin-linux-amd64.deb\n"

	if got, err := parseChecksum(sums, "eirin-linux-amd64.zip"); err != nil || got != hash {
		t.Errorf("got %q, %v", got, err)
	}
	if _, err := parseChecksum(sums, "eirin-linux-amd64.deb"); err != nil {
		t.Errorf("binary-marker line not parsed: %v", err)
	}
	if _, err := parseChecksum(sums, "missing"); err == nil {
		t.Error("expected an error for an unlisted asset")
	}
	if _, err := parseChecksum("short  eirin-linux-amd64.zip", "eirin-linux-amd64.zip"); err == nil {
		t.Error("expected an error for a malformed digest")
	}
}
