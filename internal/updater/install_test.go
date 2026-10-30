package updater

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// releaseServer serves files by name plus a matching SHA256SUMS, and returns a
// Release pointing at them.
func releaseServer(t *testing.T, files map[string][]byte) (Client, Release) {
	t.Helper()
	mux := http.NewServeMux()
	var rel Release
	rel.Version = "9.9.9"
	sums := ""
	for name, body := range files {
		mux.HandleFunc("/"+name, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) })
		sums += sum(body) + "  " + name + "\n"
	}
	mux.HandleFunc("/"+checksumsAsset, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(sums)) })
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	for name := range files {
		rel.Assets = append(rel.Assets, Asset{Name: name, URL: srv.URL + "/" + name})
	}
	rel.Assets = append(rel.Assets, Asset{Name: checksumsAsset, URL: srv.URL + "/" + checksumsAsset})
	return Client{}, rel
}

func TestInstallAppImage(t *testing.T) {
	target := filepath.Join(t.TempDir(), "Eirin.AppImage")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	inst := Install{Kind: KindAppImage, Path: target}
	c, rel := releaseServer(t, map[string][]byte{inst.assetName(): []byte("new appimage")})

	var phases []string
	err := c.Install(context.Background(), rel, inst, func(p Progress) { phases = append(phases, p.Phase) })
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(target); string(got) != "new appimage" {
		t.Errorf("target = %q", got)
	}
	if phases[len(phases)-1] != "installing" {
		t.Errorf("phases = %v", phases)
	}
}

func TestInstallBinaryFromZip(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "eirin")
	if err := os.WriteFile(target, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(t.TempDir(), "r.zip")
	writeZip(t, zipPath, map[string]string{"eirin": "new binary"})
	zipBytes, _ := os.ReadFile(zipPath)

	inst := Install{Kind: KindBinary, Path: target}
	c, rel := releaseServer(t, map[string][]byte{inst.assetName(): zipBytes})
	if err := c.Install(context.Background(), rel, inst, func(Progress) {}); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(target); string(got) != "new binary" {
		t.Errorf("target = %q", got)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Errorf("temp files left in install dir: %v", entries)
	}
}

func TestInstallDeb(t *testing.T) {
	if DebPrerequisites() != "" {
		t.Skip("pkexec/apt-get not available")
	}
	inst := Install{Kind: KindDeb, Path: "/usr/bin/eirin"}
	c, rel := releaseServer(t, map[string][]byte{inst.assetName(): []byte("deb bytes")})

	var installed string
	orig := runDebInstall
	runDebInstall = func(_ context.Context, debPath string) error {
		b, err := os.ReadFile(debPath)
		installed = string(b)
		return err
	}
	t.Cleanup(func() { runDebInstall = orig })

	if err := c.Install(context.Background(), rel, inst, func(Progress) {}); err != nil {
		t.Fatal(err)
	}
	if installed != "deb bytes" {
		t.Errorf("apt saw %q", installed)
	}
}

func TestInstallRefusals(t *testing.T) {
	target := filepath.Join(t.TempDir(), "Eirin.AppImage")
	inst := Install{Kind: KindAppImage, Path: target}
	body := []byte("data")

	// No checksums in the release: refuse rather than install unverified.
	c, rel := releaseServer(t, map[string][]byte{inst.assetName(): body})
	var noSums Release
	for _, a := range rel.Assets {
		if a.Name != checksumsAsset {
			noSums.Assets = append(noSums.Assets, a)
		}
	}
	if err := c.Install(context.Background(), noSums, inst, func(Progress) {}); err == nil {
		t.Error("expected a refusal without SHA256SUMS")
	}

	// Asset missing for this platform.
	if err := c.Install(context.Background(), Release{}, inst, func(Progress) {}); err == nil {
		t.Error("expected an error for a missing asset")
	}

	// Corrupt download: nothing installed, nothing left behind.
	bad := Release{Assets: rel.Assets}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("tampered")) }))
	defer srv.Close()
	for i := range bad.Assets {
		if bad.Assets[i].Name == inst.assetName() {
			bad.Assets[i].URL = srv.URL
		}
	}
	if err := c.Install(context.Background(), bad, inst, func(Progress) {}); err == nil {
		t.Error("expected a checksum failure")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("target was written despite the failed verification")
	}
	if entries, _ := os.ReadDir(filepath.Dir(target)); len(entries) != 0 {
		t.Errorf("temp files left behind: %v", entries)
	}

	if err := c.Install(context.Background(), rel, Install{Kind: KindUnsupported}, func(Progress) {}); err == nil {
		t.Error("expected an error for an unsupported platform")
	}
}
