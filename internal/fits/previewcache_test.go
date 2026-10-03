package fits

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestPreviewCacheReusesUntilFileChanges(t *testing.T) {
	path := writeBayerSky(t, 128, 96)
	var c PreviewCache

	first, err := c.Preview(path, 64, 2)
	if err != nil {
		t.Fatal(err)
	}
	if c.order.Len() != 1 {
		t.Fatalf("cache holds %d entries, want 1", c.order.Len())
	}
	again, _ := c.Preview(path, 64, 2)
	if again != first || c.order.Len() != 1 {
		t.Error("second call should be served from the cache")
	}
	if _, err := c.Preview(path, 64, 3); err != nil || c.order.Len() != 2 {
		t.Errorf("another stretch level is a separate entry (len %d, err %v)", c.order.Len(), err)
	}

	// Touching the file invalidates its entries.
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Preview(path, 64, 2); err != nil || c.order.Len() != 3 {
		t.Errorf("changed file should be regenerated (len %d, err %v)", c.order.Len(), err)
	}
}

func TestPreviewCacheEvictsOldest(t *testing.T) {
	var c PreviewCache
	big := strings.Repeat("x", previewCacheBytes/2+1)
	c.put("a", big)
	c.put("b", big)
	if _, ok := c.get("a"); ok {
		t.Error("oldest entry should have been evicted")
	}
	if _, ok := c.get("b"); !ok {
		t.Error("newest entry should stay")
	}
	if c.bytes != len(big) {
		t.Errorf("bytes = %d, want %d", c.bytes, len(big))
	}
}

func TestPreviewCacheMissingFile(t *testing.T) {
	var c PreviewCache
	if _, err := c.Preview("/nonexistent/x.fit", 64, 2); err == nil {
		t.Error("want an error for a missing file")
	}
}
