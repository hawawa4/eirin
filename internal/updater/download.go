package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"time"
)

// Progress reports an install in flight. Phase is "downloading", "installing"
// or "done"; Done/Total are bytes while downloading.
type Progress struct {
	Phase string `json:"phase"`
	Done  int64  `json:"done"`
	Total int64  `json:"total"`
}

const progressEvery = 100 * time.Millisecond

// download fetches url into dest, verifying its SHA-256 against wantSHA. dest
// is removed again if anything goes wrong, including a checksum mismatch.
func (c Client) download(ctx context.Context, asset Asset, dest, wantSHA string, report func(Progress)) (err error) {
	resp, err := c.get(ctx, asset.URL, "application/octet-stream")
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()

	total := resp.ContentLength
	if total <= 0 {
		total = asset.Size
	}

	f, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			_ = os.Remove(dest)
		}
	}()

	hash := sha256.New()
	var done int64
	var last time.Time
	buf := make([]byte, 256<<10)
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			hash.Write(buf[:n])
			done += int64(n)
			if now := time.Now(); now.Sub(last) >= progressEvery {
				last = now
				report(Progress{Phase: "downloading", Done: done, Total: total})
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	report(Progress{Phase: "downloading", Done: done, Total: total})

	if got := hex.EncodeToString(hash.Sum(nil)); got != wantSHA {
		return fmt.Errorf("checksum mismatch for %s (expected %s, got %s)", asset.Name, wantSHA, got)
	}
	return nil
}
