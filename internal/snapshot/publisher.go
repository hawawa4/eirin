package snapshot

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/TaruDesigns/eirin/internal/store"
)

// DefaultPublishInterval is how often the publisher checks for changes.
const DefaultPublishInterval = 2 * time.Minute

// PublisherStatus describes the desktop side of the snapshot.
type PublisherStatus struct {
	Enabled       bool
	Path          string // where the snapshot is written; empty without a root folder
	LastPublished time.Time
	LastError     string
}

// Publisher republishes the snapshot whenever the database has changed, while
// the snapshot_enabled preference is on. It never fails the app: errors are
// logged and reported through Status.
type Publisher struct {
	store    func() *store.Store
	interval time.Duration

	mu            sync.Mutex
	published     bool  // a snapshot was written since publishing was (re)enabled
	lastCount     int64 // store change count at the last publish
	lastPublished time.Time
	lastErr       string

	stop chan struct{}
	done chan struct{}
}

// NewPublisher returns a publisher reading from the store returned by st.
func NewPublisher(st func() *store.Store) *Publisher {
	return &Publisher{store: st, interval: DefaultPublishInterval}
}

// Start checks for changes every interval until Stop is called.
func (p *Publisher) Start() {
	p.stop = make(chan struct{})
	p.done = make(chan struct{})
	go func() {
		defer close(p.done)
		ticker := time.NewTicker(p.interval)
		defer ticker.Stop()
		p.PublishIfChanged()
		for {
			select {
			case <-ticker.C:
				p.PublishIfChanged()
			case <-p.stop:
				p.PublishIfChanged()
				return
			}
		}
	}()
}

// Stop publishes any pending changes and stops the loop, waiting at most
// until ctx is done (a slow NAS must not hang app shutdown).
func (p *Publisher) Stop(ctx context.Context) {
	if p.stop == nil {
		return
	}
	close(p.stop)
	select {
	case <-p.done:
	case <-ctx.Done():
		slog.Warn("snapshot: gave up waiting for the final publish")
	}
}

// PublishIfChanged publishes when enabled and the database changed since the
// last publish (or nothing was published yet). It reports whether it published.
func (p *Publisher) PublishIfChanged() bool {
	st := p.store()
	if st == nil {
		return false
	}
	prefs := st.Load()
	p.mu.Lock()
	defer p.mu.Unlock()
	if !prefs.SnapshotEnabled || prefs.RootFolder == "" {
		p.published = false
		return false
	}
	count, err := st.ChangeCount()
	if err != nil {
		p.fail(err)
		return false
	}
	if p.published && count == p.lastCount {
		return false
	}
	return p.publishLocked(st, prefs.RootFolder, count) == nil
}

// PublishNow publishes immediately, whether or not anything changed or
// publishing is enabled.
func (p *Publisher) PublishNow() error {
	st := p.store()
	if st == nil {
		return nil
	}
	root := st.Load().RootFolder
	p.mu.Lock()
	defer p.mu.Unlock()
	count, err := st.ChangeCount()
	if err != nil {
		p.fail(err)
		return err
	}
	return p.publishLocked(st, root, count)
}

// Status returns the current state for the settings UI.
func (p *Publisher) Status() PublisherStatus {
	var prefs store.Prefs
	if st := p.store(); st != nil {
		prefs = st.Load()
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	s := PublisherStatus{
		Enabled:       prefs.SnapshotEnabled,
		LastPublished: p.lastPublished,
		LastError:     p.lastErr,
	}
	if prefs.RootFolder != "" {
		s.Path = Path(prefs.RootFolder)
	}
	return s
}

func (p *Publisher) publishLocked(st *store.Store, root string, count int64) error {
	if err := Publish(st, root); err != nil {
		p.fail(err)
		return err
	}
	p.published = true
	p.lastCount = count
	p.lastPublished = time.Now()
	p.lastErr = ""
	slog.Info("snapshot: published", "path", Path(root))
	return nil
}

func (p *Publisher) fail(err error) {
	p.lastErr = err.Error()
	slog.Warn("snapshot: publish failed", "err", err)
}
