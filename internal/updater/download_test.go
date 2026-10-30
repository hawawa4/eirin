package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func TestDownload(t *testing.T) {
	payload := []byte("pretend this is an AppImage")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(payload)
	}))
	defer srv.Close()
	asset := Asset{Name: "x", URL: srv.URL}

	t.Run("verified", func(t *testing.T) {
		dest := filepath.Join(t.TempDir(), "out")
		var last Progress
		err := Client{}.download(context.Background(), asset, dest, sum(payload), func(p Progress) { last = p })
		if err != nil {
			t.Fatal(err)
		}
		if got, _ := os.ReadFile(dest); string(got) != string(payload) {
			t.Errorf("file contents differ: %q", got)
		}
		if last.Done != int64(len(payload)) {
			t.Errorf("final progress = %+v", last)
		}
	})

	t.Run("checksum mismatch removes the file", func(t *testing.T) {
		dest := filepath.Join(t.TempDir(), "out")
		err := Client{}.download(context.Background(), asset, dest, sum([]byte("other")), func(Progress) {})
		if err == nil {
			t.Fatal("expected a checksum error")
		}
		if _, statErr := os.Stat(dest); !os.IsNotExist(statErr) {
			t.Error("corrupt download was left on disk")
		}
	})
}
