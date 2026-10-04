package snapshot

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/TaruDesigns/eirin/internal/store"
)

func newTestPublisher(t *testing.T, root string, enabled bool) (*Publisher, *store.Store) {
	t.Helper()
	s := desktopStore(t, root)
	if enabled {
		must(t, s.Set(store.KeySnapshotEnabled, "true"))
	}
	return NewPublisher(func() *store.Store { return s }), s
}

func TestPublisherDisabledDoesNothing(t *testing.T) {
	root := t.TempDir()
	p, _ := newTestPublisher(t, root, false)
	if p.PublishIfChanged() {
		t.Error("published while disabled")
	}
	if _, err := os.Stat(Path(root)); err == nil {
		t.Error("snapshot written while disabled")
	}
}

func TestPublisherPublishesOnlyOnChange(t *testing.T) {
	root := t.TempDir()
	p, s := newTestPublisher(t, root, true)

	if !p.PublishIfChanged() {
		t.Fatal("first check didn't publish")
	}
	if p.PublishIfChanged() {
		t.Error("published again without changes")
	}
	must(t, s.RejectFrame(filepath.Join(root, "M31", "stack.fits"), ""))
	if !p.PublishIfChanged() {
		t.Error("didn't publish after a change")
	}
	st := p.Status()
	if !st.Enabled || st.Path != Path(root) || st.LastPublished.IsZero() || st.LastError != "" {
		t.Errorf("Status() = %+v", st)
	}
}

func TestPublisherRepublishesAfterReenabling(t *testing.T) {
	root := t.TempDir()
	p, s := newTestPublisher(t, root, true)
	p.PublishIfChanged()
	must(t, s.Set(store.KeySnapshotEnabled, "false"))
	p.PublishIfChanged()
	must(t, os.Remove(Path(root)))
	must(t, s.Set(store.KeySnapshotEnabled, "true"))
	if !p.PublishIfChanged() {
		t.Error("didn't publish after being re-enabled")
	}
}

func TestPublisherRecordsErrors(t *testing.T) {
	root := t.TempDir()
	p, _ := newTestPublisher(t, root, true)
	// A file where the .eirin directory should be makes publishing fail.
	must(t, os.WriteFile(filepath.Join(root, DirName), nil, 0o644))

	if p.PublishIfChanged() {
		t.Fatal("publish reported success")
	}
	if p.Status().LastError == "" {
		t.Error("error not recorded")
	}
	if err := p.PublishNow(); err == nil {
		t.Error("PublishNow returned no error")
	}
}

func TestPublishNowIgnoresEnabledFlag(t *testing.T) {
	root := t.TempDir()
	p, _ := newTestPublisher(t, root, false)
	must(t, p.PublishNow())
	if _, err := os.Stat(Path(root)); err != nil {
		t.Errorf("PublishNow didn't write the snapshot: %v", err)
	}
}

func TestPublisherStopPublishesPendingChanges(t *testing.T) {
	root := t.TempDir()
	p, _ := newTestPublisher(t, root, true)
	p.Start()
	p.Stop(context.Background())
	if _, err := os.Stat(Path(root)); err != nil {
		t.Errorf("no snapshot after Start/Stop: %v", err)
	}
}
