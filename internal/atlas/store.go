package atlas

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// MaxBytes caps the size of the data file.
	MaxBytes = 64 << 20
	// RetryPause is the shortest wait between two failed first loads.
	RetryPause = 5 * time.Second
)

// buildSnapshot is Build; tests swap it to prove a panic is contained.
var buildSnapshot = Build

// Options configures a Store.
type Options struct {
	URL       string        // data file URL; ignored when File is set
	File      string        // local copy; when set, no network and no disk cache
	CacheDir  string        // disk cache directory; "" disables it
	TTL       time.Duration // how long a copy is served before asking again; 0 = never
	Timeout   time.Duration // limit for one download
	UserAgent string
	Logf      func(format string, args ...any)
}

// Status says where the served copy came from and how fresh it is.
type Status struct {
	Source       string
	LoadedAt     time.Time
	CheckedAt    time.Time
	Stale        bool
	RefreshError string
}

// Store owns the current snapshot and keeps it fresh.
type Store struct {
	opt        Options
	client     *http.Client
	now        func() time.Time
	maxBytes   int64
	retryPause time.Duration

	snap atomic.Pointer[Snapshot]

	mu          sync.Mutex
	loading     chan struct{}
	lastErr     error
	lastAttempt time.Time
	checkedAt   time.Time
	nextCheck   time.Time
	fileMod     time.Time
	fileSize    int64
}

// NewStore returns a store that has loaded nothing yet.
func NewStore(opt Options) *Store {
	if opt.Logf == nil {
		opt.Logf = func(string, ...any) {}
	}
	if opt.Timeout <= 0 {
		opt.Timeout = 30 * time.Second
	}
	return &Store{
		opt:        opt,
		client:     &http.Client{Timeout: opt.Timeout},
		now:        time.Now,
		maxBytes:   MaxBytes,
		retryPause: RetryPause,
	}
}

// Source is the file path or URL the store reads.
func (s *Store) Source() string {
	if s.opt.File != "" {
		return s.opt.File
	}
	return s.opt.URL
}

// Start begins the first load in the background. It never blocks.
func (s *Store) Start() {
	s.mu.Lock()
	s.beginLocked(true)
	s.mu.Unlock()
}

// Snapshot returns the snapshot to answer one call from. A fresh copy comes
// back at once; an old one comes back at once and triggers one background
// refresh; with no copy at all it waits for a load, bounded by ctx.
func (s *Store) Snapshot(ctx context.Context) (*Snapshot, error) {
	if sn := s.snap.Load(); sn != nil {
		s.maybeRefresh()
		return sn, nil
	}
	s.mu.Lock()
	if s.loading == nil && s.lastErr != nil && s.now().Sub(s.lastAttempt) < s.retryPause {
		err := s.lastErr
		s.mu.Unlock()
		return nil, s.unavailable(err)
	}
	ch := s.beginLocked(true)
	s.mu.Unlock()
	select {
	case <-ch:
	case <-ctx.Done():
		return nil, s.unavailable(ctx.Err())
	}
	if sn := s.snap.Load(); sn != nil {
		return sn, nil
	}
	s.mu.Lock()
	err := s.lastErr
	s.mu.Unlock()
	return nil, s.unavailable(err)
}

func (s *Store) unavailable(err error) error {
	if err == nil {
		err = errors.New("no data loaded")
	}
	return fmt.Errorf("the atlas data is not available: %v (source %s). The next call tries again", err, s.Source())
}

// Status reports the freshness of the served copy.
func (s *Store) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := Status{Source: s.Source(), CheckedAt: s.checkedAt}
	if sn := s.snap.Load(); sn != nil {
		st.LoadedAt = sn.LoadedAt
		if s.lastErr != nil {
			st.Stale = true
			st.RefreshError = s.lastErr.Error()
		}
	}
	return st
}

func (s *Store) maybeRefresh() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.opt.TTL > 0 && s.loading == nil && !s.now().Before(s.nextCheck) {
		s.beginLocked(false)
	}
}

// beginLocked starts a load unless one runs, and returns its done channel.
func (s *Store) beginLocked(initial bool) chan struct{} {
	if s.loading != nil {
		return s.loading
	}
	ch := make(chan struct{})
	s.loading = ch
	go func() {
		stale := s.load(initial)
		s.mu.Lock()
		s.loading = nil
		s.mu.Unlock()
		close(ch)
		if stale {
			// The disk copy is served already; refresh it in the background
			// so no caller waits for the download.
			s.maybeRefresh()
		}
	}()
	return ch
}

// waitIdle waits for a running load to finish. Tests use it.
func (s *Store) waitIdle() {
	s.mu.Lock()
	ch := s.loading
	s.mu.Unlock()
	if ch != nil {
		<-ch
	}
}

// load runs one load. It returns true when it published an old disk copy
// and left the download to a background refresh.
func (s *Store) load(initial bool) bool {
	if s.opt.File != "" {
		s.loadFile()
		return false
	}
	if initial && s.snap.Load() == nil && s.opt.CacheDir != "" {
		s.loadCache()
		if s.snap.Load() != nil {
			return true
		}
	}
	s.fetch()
	return false
}

func (s *Store) fail(err error) {
	s.mu.Lock()
	s.lastErr = err
	s.lastAttempt = s.now()
	s.nextCheck = s.lastAttempt.Add(s.opt.TTL)
	s.mu.Unlock()
	if s.snap.Load() != nil {
		s.opt.Logf("refresh failed, still serving the copy loaded before: %v", err)
	} else {
		s.opt.Logf("load failed: %v", err)
	}
}

func (s *Store) succeed(at time.Time) {
	s.mu.Lock()
	s.lastErr = nil
	s.lastAttempt = at
	s.checkedAt = at
	s.nextCheck = at.Add(s.opt.TTL)
	s.mu.Unlock()
}

func (s *Store) sizeError() error {
	if s.maxBytes%(1<<20) == 0 {
		return fmt.Errorf("data file is larger than %d MB", s.maxBytes>>20)
	}
	return fmt.Errorf("data file is larger than %d bytes", s.maxBytes)
}

// build decodes, validates and indexes a body.
// A panic while decoding or indexing a malformed file becomes an error, so
// a bad file never takes the server down and the copy served before stays.
func (s *Store) build(body []byte, etag, source string, at time.Time) (sn *Snapshot, err error) {
	defer func() {
		if p := recover(); p != nil {
			sn, err = nil, fmt.Errorf("the data file could not be read: %v", p)
		}
	}()
	f, err := Decode(body)
	if err != nil {
		return nil, err
	}
	sn = buildSnapshot(f)
	sn.ETag, sn.Source, sn.LoadedAt = etag, source, at
	return sn, nil
}

func (s *Store) publish(sn *Snapshot) {
	s.snap.Store(sn)
	f := sn.File
	s.opt.Logf("loaded %s generated %s from %s: %d people, %d places, %d events; %d dangling references skipped, %d citations and %d event passages not parsed",
		sn.Format, sn.Generated, sn.Source, len(f.People), len(f.Places), len(f.Events),
		sn.Stats.Dangling, sn.Stats.BadCitations, sn.Stats.BadEventPassages)
	for _, w := range sn.Warnings() {
		s.opt.Logf("warning: %s", w)
	}
}

func (s *Store) fetch() {
	at := s.now()
	ctx, cancel := context.WithTimeout(context.Background(), s.opt.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.opt.URL, nil)
	if err != nil {
		s.fail(err)
		return
	}
	req.Header.Set("User-Agent", s.opt.UserAgent)
	cur := s.snap.Load()
	if cur != nil && cur.ETag != "" {
		req.Header.Set("If-None-Match", cur.ETag)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		s.fail(err)
		return
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotModified && cur != nil:
		s.succeed(at)
		return
	case resp.StatusCode != http.StatusOK:
		s.fail(fmt.Errorf("%s answered HTTP %d", s.opt.URL, resp.StatusCode))
		return
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, s.maxBytes+1))
	if err != nil {
		s.fail(fmt.Errorf("reading the data file: %w", err))
		return
	}
	if int64(len(body)) > s.maxBytes {
		s.fail(s.sizeError())
		return
	}
	etag := resp.Header.Get("ETag")
	sn, err := s.build(body, etag, s.opt.URL, at)
	if err != nil {
		s.fail(err)
		return
	}
	s.publish(sn)
	s.succeed(at)
	s.writeCache(body, etag, at)
}

func (s *Store) loadFile() {
	at := s.now()
	fi, err := os.Stat(s.opt.File)
	if err != nil {
		s.fail(err)
		return
	}
	s.mu.Lock()
	same := s.snap.Load() != nil && fi.ModTime().Equal(s.fileMod) && fi.Size() == s.fileSize
	s.mu.Unlock()
	if same {
		s.succeed(at)
		return
	}
	if fi.Size() > s.maxBytes {
		s.fail(s.sizeError())
		return
	}
	body, err := os.ReadFile(s.opt.File)
	if err != nil {
		s.fail(err)
		return
	}
	sn, err := s.build(body, "", s.opt.File, at)
	if err != nil {
		s.fail(err)
		return
	}
	s.mu.Lock()
	s.fileMod, s.fileSize = fi.ModTime(), fi.Size()
	s.mu.Unlock()
	s.publish(sn)
	s.succeed(at)
}
