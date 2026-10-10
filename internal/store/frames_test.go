package store

import (
	"strings"
	"testing"
)

// ── ClassifyFrameType ─────────────────────────────────────────────────────────

func TestClassifyFrameType(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		// Directory ending in _sub overrides filename — always light.
		{"/data/NGC1234_sub/Dark_001.fits", FrameTypeLight},
		{"/data/some_sub/Light_001.fits", FrameTypeLight},
		// Filename prefix detection.
		{"/data/Light_001.fits", FrameTypeLight},
		{"/data/Dark_001.fits", FrameTypeDark},
		{"/data/Flat_001.fits", FrameTypeFlat},
		{"/data/Bias_001.fits", FrameTypeBias},
		{"/data/bias_001.fits", FrameTypeBias}, // case-insensitive
		{"/data/BIAS_001.fits", FrameTypeBias},
		// PROCESSED / FINAL / HERO as a word in the name: processed.
		{"/data/processed_output.fits", FrameTypeProcessed},
		{"/data/M31_FINAL.fit", FrameTypeProcessed},
		{"/data/NGC7000-hero-v2.fits", FrameTypeProcessed},
		{"/data/M31Final.fit", FrameTypeProcessed},
		{"/data/final/result.fits", FrameTypeStacked}, // only the file name counts
		{"/data/m31_unprocessed.fits", FrameTypeStacked},
		{"/data/finally.fits", FrameTypeStacked},
		{"/data/Light_final_001.fits", FrameTypeLight}, // prefixes win
		// Anything else defaults to stacked.
		{"/data/result.fits", FrameTypeStacked},
	}

	for _, tt := range tests {
		got := ClassifyFrameType(tt.path)
		if got != tt.want {
			t.Errorf("ClassifyFrameType(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestClassifyRasterType(t *testing.T) {
	tests := map[string]string{
		"/data/M31_final.png":     FrameTypeProcessed,
		"/data/HERO.jpg":          FrameTypeProcessed,
		"/data/m42 processed.tif": FrameTypeProcessed,
		"/data/m42.png":           FrameTypeImage,
		"/data/heron.jpg":         FrameTypeImage,
	}
	for path, want := range tests {
		if got := ClassifyRasterType(path); got != want {
			t.Errorf("ClassifyRasterType(%q) = %q, want %q", path, got, want)
		}
	}
}

// ── Frame DB operations ───────────────────────────────────────────────────────

func TestUpsertAndGetFrames(t *testing.T) {
	s := newTestStore(t)

	path := "/nas/NGC1234/Light_001.fits"
	f := Frame{
		FileSize:  4096,
		Object:    "NGC1234",
		Filter:    "Ha",
		ExpTime:   300.0,
		FrameType: FrameTypeLight,
	}
	if err := s.UpsertFrame(path, f); err != nil {
		t.Fatalf("UpsertFrame: %v", err)
	}

	frames, err := s.GetFrames([]string{path})
	if err != nil {
		t.Fatalf("GetFrames: %v", err)
	}
	got, ok := frames[path]
	if !ok {
		t.Fatal("frame not found after upsert")
	}
	if got.Object != "NGC1234" {
		t.Errorf("Object = %q, want NGC1234", got.Object)
	}
	if got.Filter != "Ha" {
		t.Errorf("Filter = %q, want Ha", got.Filter)
	}
	if got.ExpTime != 300.0 {
		t.Errorf("ExpTime = %f, want 300.0", got.ExpTime)
	}
	if got.CachedAt == 0 {
		t.Error("CachedAt should be non-zero after upsert")
	}
}

func TestUpsertPreservesRejectionOnConflict(t *testing.T) {
	s := newTestStore(t)

	path := "/nas/NGC1234/Light_001.fits"
	if err := s.RejectFrame(path, "trailing gradient"); err != nil {
		t.Fatalf("RejectFrame: %v", err)
	}

	// Upsert should update header data but leave rejection intact.
	if err := s.UpsertFrame(path, Frame{Object: "NGC1234", FrameType: FrameTypeLight}); err != nil {
		t.Fatalf("UpsertFrame: %v", err)
	}

	frames, _ := s.GetFrames([]string{path})
	got := frames[path]
	if !got.Rejected {
		t.Error("Rejected flag should be preserved after upsert")
	}
	if got.RejectionReason != "trailing gradient" {
		t.Errorf("RejectionReason = %q, want %q", got.RejectionReason, "trailing gradient")
	}
	if got.Object != "NGC1234" {
		t.Errorf("header should be updated: Object = %q, want NGC1234", got.Object)
	}
}

func TestGetFramesEmpty(t *testing.T) {
	s := newTestStore(t)
	frames, err := s.GetFrames([]string{})
	if err != nil {
		t.Fatalf("GetFrames([]): %v", err)
	}
	if len(frames) != 0 {
		t.Errorf("expected empty result, got %d frames", len(frames))
	}
}

func TestGetFramesMissingPathIgnored(t *testing.T) {
	s := newTestStore(t)
	frames, err := s.GetFrames([]string{"/nonexistent/path.fits"})
	if err != nil {
		t.Fatalf("GetFrames: %v", err)
	}
	if len(frames) != 0 {
		t.Errorf("expected 0 results for unknown path, got %d", len(frames))
	}
}

func TestBatchUpsertFrames(t *testing.T) {
	s := newTestStore(t)

	entries := map[string]Frame{
		"/nas/obj/Light_001.fits": {Object: "M42", Filter: "Ha", FrameType: FrameTypeLight},
		"/nas/obj/Light_002.fits": {Object: "M42", Filter: "OIII", FrameType: FrameTypeLight},
		"/nas/obj/Dark_001.fits":  {FrameType: FrameTypeDark},
	}
	if err := s.BatchUpsertFrames(entries); err != nil {
		t.Fatalf("BatchUpsertFrames: %v", err)
	}

	paths := make([]string, 0, len(entries))
	for k := range entries {
		paths = append(paths, k)
	}
	frames, err := s.GetFrames(paths)
	if err != nil {
		t.Fatalf("GetFrames: %v", err)
	}
	if len(frames) != 3 {
		t.Errorf("got %d frames, want 3", len(frames))
	}
}

func TestBatchUpsertFramesEmpty(t *testing.T) {
	s := newTestStore(t)
	if err := s.BatchUpsertFrames(nil); err != nil {
		t.Errorf("BatchUpsertFrames(nil): %v", err)
	}
	if err := s.BatchUpsertFrames(map[string]Frame{}); err != nil {
		t.Errorf("BatchUpsertFrames({}): %v", err)
	}
}

func TestGetAllFramesUnder(t *testing.T) {
	s := newTestStore(t)

	// Indexed frame under target root.
	must(t, s.UpsertFrame("/nas/root/a/Light_001.fits", Frame{Object: "M42", FrameType: FrameTypeLight}))
	// Un-indexed (rejection-only row, cached_at = 0).
	must(t, s.RejectFrame("/nas/root/a/Light_002.fits", "blurry"))
	// Indexed frame under a different root — must not appear.
	must(t, s.UpsertFrame("/nas/other/Light_003.fits", Frame{Object: "M51", FrameType: FrameTypeLight}))

	frames, err := s.GetAllFramesUnder("/nas/root")
	if err != nil {
		t.Fatalf("GetAllFramesUnder: %v", err)
	}
	if len(frames) != 1 {
		t.Errorf("got %d frames, want 1 (only cached)", len(frames))
	}
	if len(frames) > 0 && frames[0].NasPath != "/nas/root/a/Light_001.fits" {
		t.Errorf("unexpected NasPath %q", frames[0].NasPath)
	}
}

func TestGetFramesUnderByType(t *testing.T) {
	s := newTestStore(t)
	must(t, s.UpsertFrame("/nas/root/darks/Dark_002.fits", Frame{Object: "B", FrameType: FrameTypeDark}))
	must(t, s.UpsertFrame("/nas/root/darks/Dark_001.fits", Frame{Object: "A", FrameType: FrameTypeDark}))
	must(t, s.UpsertFrame("/nas/root/a/Light_001.fits", Frame{Object: "M42", FrameType: FrameTypeLight}))
	must(t, s.UpsertFrame("/nas/root/flats/Flat_001.fits", Frame{FrameType: FrameTypeFlat}))
	must(t, s.UpsertFrame("/nas/other/Dark_003.fits", Frame{FrameType: FrameTypeDark}))
	must(t, s.RejectFrame("/nas/root/darks/Dark_004.fits", "rejection-only row"))

	frames, err := s.GetFramesUnderByType("/nas/root", FrameTypeDark)
	if err != nil {
		t.Fatalf("GetFramesUnderByType: %v", err)
	}
	var got []string
	for _, f := range frames {
		got = append(got, f.NasPath)
	}
	want := []string{"/nas/root/darks/Dark_001.fits", "/nas/root/darks/Dark_002.fits"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("darks = %v, want %v (sorted by object)", got, want)
	}

	if frames, err := s.GetFramesUnderByType("/nas/root", FrameTypeBias); err != nil || len(frames) != 0 {
		t.Errorf("biases = %v err = %v, want none", frames, err)
	}
}

func TestGetCoverageLights(t *testing.T) {
	s := newTestStore(t)
	must(t, s.UpsertFrame("/nas/root/M42_sub/Light_001.fit", Frame{Object: "M 42", FrameType: FrameTypeLight}))
	must(t, s.UpsertFrame("/nas/root/M42_sub/Light_002.fit", Frame{Object: "M 42", FrameType: FrameTypeLight}))
	must(t, s.UpsertFrame("/nas/root/M42/Stacked_M42.fit", Frame{Object: "M 42", FrameType: FrameTypeStacked}))
	must(t, s.UpsertFrame("/nas/other/Light_003.fit", Frame{Object: "M 42", FrameType: FrameTypeLight}))
	must(t, s.RejectFrame("/nas/root/M42_sub/Light_002.fit", "clouds"))

	frames, err := s.GetCoverageLights("/nas/root")
	if err != nil {
		t.Fatalf("GetCoverageLights: %v", err)
	}
	if len(frames) != 1 || frames[0].NasPath != "/nas/root/M42_sub/Light_001.fit" {
		t.Errorf("got %+v, want only the non-rejected light under the root", frames)
	}
}

func TestRejectAndUnrejectFrame(t *testing.T) {
	s := newTestStore(t)

	path := "/nas/root/Light_001.fits"
	must(t, s.UpsertFrame(path, Frame{Object: "NGC1", FrameType: FrameTypeLight}))

	if err := s.RejectFrame(path, "satellite trail"); err != nil {
		t.Fatalf("RejectFrame: %v", err)
	}
	frames, _ := s.GetFrames([]string{path})
	if !frames[path].Rejected {
		t.Error("frame should be rejected")
	}
	if frames[path].RejectionReason != "satellite trail" {
		t.Errorf("RejectionReason = %q, want %q", frames[path].RejectionReason, "satellite trail")
	}

	if err := s.UnrejectFrame(path); err != nil {
		t.Fatalf("UnrejectFrame: %v", err)
	}
	frames, _ = s.GetFrames([]string{path})
	if frames[path].Rejected {
		t.Error("frame should not be rejected after unreject")
	}
}

func TestRejectFrameCreatesRowIfMissing(t *testing.T) {
	s := newTestStore(t)

	path := "/nas/root/new.fits"
	if err := s.RejectFrame(path, "test reason"); err != nil {
		t.Fatalf("RejectFrame on new path: %v", err)
	}
	frames, _ := s.GetFrames([]string{path})
	got, ok := frames[path]
	if !ok {
		t.Fatal("frame should exist after RejectFrame even if never upserted")
	}
	if !got.Rejected {
		t.Error("frame should be rejected")
	}
	if got.CachedAt != 0 {
		t.Error("CachedAt should remain 0 for reject-only rows")
	}
}

func TestGetAllRejectedUnder(t *testing.T) {
	s := newTestStore(t)

	must(t, s.RejectFrame("/nas/root/a.fits", "bad"))
	must(t, s.RejectFrame("/nas/root/b.fits", "blurry"))
	must(t, s.UpsertFrame("/nas/root/c.fits", Frame{FrameType: FrameTypeLight})) // not rejected
	must(t, s.RejectFrame("/nas/other/d.fits", "test"))                          // different root

	paths, err := s.GetAllRejectedUnder("/nas/root")
	if err != nil {
		t.Fatalf("GetAllRejectedUnder: %v", err)
	}
	if len(paths) != 2 {
		t.Errorf("got %d paths, want 2", len(paths))
	}
}

func TestUpdateFrameQuality(t *testing.T) {
	s := newTestStore(t)

	path := "/nas/root/Light_001.fits"
	must(t, s.UpsertFrame(path, Frame{FrameType: FrameTypeLight}))

	q := FrameQuality{
		FWHM:       3.5,
		FWHMUnit:   "px",
		Background: 1200.0,
		Noise:      12.0,
		SNR:        100.0,
		StarCount:  350,
	}
	if err := s.UpdateFrameQuality(path, q); err != nil {
		t.Fatalf("UpdateFrameQuality: %v", err)
	}

	frames, _ := s.GetFrames([]string{path})
	got := frames[path]
	if !got.QualityAnalyzed {
		t.Error("QualityAnalyzed should be true")
	}
	if got.FWHM == nil || *got.FWHM != 3.5 {
		t.Errorf("FWHM = %v, want 3.5", got.FWHM)
	}
	if got.FWHMUnit != "px" {
		t.Errorf("FWHMUnit = %q, want px", got.FWHMUnit)
	}
	if got.StarCount == nil || *got.StarCount != 350 {
		t.Errorf("StarCount = %v, want 350", got.StarCount)
	}
	if got.Background == nil || *got.Background != 1200.0 {
		t.Errorf("Background = %v, want 1200.0", got.Background)
	}
}

func TestUpdateWCS(t *testing.T) {
	s := newTestStore(t)

	path := "/nas/root/Light_001.fits"
	must(t, s.UpsertFrame(path, Frame{FrameType: FrameTypeLight}))

	wcs := WCSResult{RA: 101.46, Dec: -20.78, PixelScale: 3.67, Rotation: 180.17}
	if err := s.UpdateWCS(path, wcs); err != nil {
		t.Fatalf("UpdateWCS: %v", err)
	}

	frames, _ := s.GetFrames([]string{path})
	got := frames[path]
	if !got.WCSSolved {
		t.Error("WCSSolved should be true")
	}
	if got.RA == nil || *got.RA != 101.46 {
		t.Errorf("RA = %v, want 101.46", got.RA)
	}
	if got.Dec == nil || *got.Dec != -20.78 {
		t.Errorf("Dec = %v, want -20.78", got.Dec)
	}
	if got.PixelScale == nil || *got.PixelScale != 3.67 {
		t.Errorf("PixelScale = %v, want 3.67", got.PixelScale)
	}
}

func TestDeleteFrame(t *testing.T) {
	s := newTestStore(t)

	path := "/nas/root/Light_001.fits"
	must(t, s.UpsertFrame(path, Frame{FrameType: FrameTypeLight}))

	if err := s.DeleteFrame(path); err != nil {
		t.Fatalf("DeleteFrame: %v", err)
	}
	frames, _ := s.GetFrames([]string{path})
	if _, ok := frames[path]; ok {
		t.Error("frame should not exist after delete")
	}
}

func TestGetAllFrameBasenames(t *testing.T) {
	s := newTestStore(t)

	must(t, s.UpsertFrame("/nas/root/Light_001.fits", Frame{FrameType: FrameTypeLight}))
	must(t, s.UpsertFrame("/nas/other/Light_002.fits", Frame{FrameType: FrameTypeLight}))

	basenames, err := s.GetAllFrameBasenames()
	if err != nil {
		t.Fatalf("GetAllFrameBasenames: %v", err)
	}
	if !basenames["Light_001.fits"] {
		t.Error("Light_001.fits should be in basenames")
	}
	if !basenames["Light_002.fits"] {
		t.Error("Light_002.fits should be in basenames")
	}
	if basenames["/nas/root/Light_001.fits"] {
		t.Error("full path should not be a key in basenames map")
	}
}

func TestSetFrameType(t *testing.T) {
	s := newTestStore(t)

	path := "/nas/root/output.fits"
	must(t, s.UpsertFrame(path, Frame{FrameType: FrameTypeStacked}))

	if err := s.SetFrameType(path, FrameTypeProcessed); err != nil {
		t.Fatalf("SetFrameType: %v", err)
	}
	frames, _ := s.GetFrames([]string{path})
	if frames[path].FrameType != FrameTypeProcessed {
		t.Errorf("FrameType = %q, want %q", frames[path].FrameType, FrameTypeProcessed)
	}
}

func TestGetFramePathsByHash(t *testing.T) {
	s := newTestStore(t)
	must(t, s.UpsertFrame("/nas/a.fits", Frame{FrameType: FrameTypeLight, FileHash: "h1"}))
	must(t, s.UpsertFrame("/nas/b.fits", Frame{FrameType: FrameTypeLight, FileHash: "h1"}))
	must(t, s.UpsertFrame("/nas/c.fits", Frame{FrameType: FrameTypeLight, FileHash: "h2"}))

	paths, err := s.GetFramePathsByHash("h1")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 {
		t.Errorf("paths = %v, want 2 entries", paths)
	}
	if paths, _ := s.GetFramePathsByHash("missing"); len(paths) != 0 {
		t.Errorf("unknown hash returned %v", paths)
	}
	if paths, _ := s.GetFramePathsByHash(""); len(paths) != 0 {
		t.Errorf("empty hash returned %v", paths)
	}
}

// ── HasFrame / FrameCount ─────────────────────────────────────────────────────

func TestHasFrameAndFrameCount(t *testing.T) {
	s := newTestStore(t)
	if n, err := s.FrameCount(); err != nil || n != 0 {
		t.Fatalf("FrameCount() on empty store = %d, %v", n, err)
	}
	must(t, s.UpsertFrame("/nas/a.fits", Frame{FrameType: FrameTypeStacked}))
	must(t, s.UpsertFrame("/nas/b.fits", Frame{FrameType: FrameTypeLight}))

	if ok, err := s.HasFrame("/nas/a.fits"); err != nil || !ok {
		t.Errorf("HasFrame(known) = %v, %v", ok, err)
	}
	if ok, err := s.HasFrame("/nas/nope.fits"); err != nil || ok {
		t.Errorf("HasFrame(unknown) = %v, %v", ok, err)
	}
	if n, err := s.FrameCount(); err != nil || n != 2 {
		t.Errorf("FrameCount() = %d, %v, want 2", n, err)
	}
}
