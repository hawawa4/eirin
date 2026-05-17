package app

import (
	"testing"

	"github.com/TaruDesigns/eirin/internal/store"
)

func TestDerefFloat(t *testing.T) {
	v := 3.14
	if got := derefFloat(&v); got != 3.14 {
		t.Errorf("derefFloat(&3.14) = %f, want 3.14", got)
	}
	if got := derefFloat(nil); got != 0 {
		t.Errorf("derefFloat(nil) = %f, want 0", got)
	}
}

func TestDerefInt64(t *testing.T) {
	v := int64(42)
	if got := derefInt64(&v); got != 42 {
		t.Errorf("derefInt64(&42) = %d, want 42", got)
	}
	if got := derefInt64(nil); got != 0 {
		t.Errorf("derefInt64(nil) = %d, want 0", got)
	}
}

func TestSetFrameTypeValidTypes(t *testing.T) {
	a := newTestApp(t)
	path := "/nas/root/output.fits"
	a.store.UpsertFrame(path, store.Frame{FrameType: store.FrameTypeStacked})

	for _, ft := range []string{
		store.FrameTypeLight, store.FrameTypeDark, store.FrameTypeFlat,
		store.FrameTypeBias, store.FrameTypeStacked, store.FrameTypeProcessed,
	} {
		if err := a.SetFrameType(path, ft); err != nil {
			t.Errorf("SetFrameType(%q): %v", ft, err)
		}
	}
}

func TestSetFrameTypeInvalidIsNoop(t *testing.T) {
	a := newTestApp(t)
	path := "/nas/root/output.fits"
	a.store.UpsertFrame(path, store.Frame{FrameType: store.FrameTypeStacked})

	// Invalid type is silently ignored (returns nil).
	if err := a.SetFrameType(path, "unknown_type"); err != nil {
		t.Errorf("SetFrameType(invalid) should return nil, got: %v", err)
	}
	// Frame type should be unchanged.
	frames, _ := a.store.GetFrames([]string{path})
	if frames[path].FrameType != store.FrameTypeStacked {
		t.Errorf("FrameType changed after invalid SetFrameType: %q", frames[path].FrameType)
	}
}
