package atlas

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Add(d time.Duration) {
	c.mu.Lock()
	c.t = c.t.Add(d)
	c.mu.Unlock()
}

// site serves one body with an ETag and honours If-None-Match.
type site struct {
	mu        sync.Mutex
	body      []byte
	etag      string
	status    int
	hits      int
	lastINM   string
	userAgent string
}

func (s *site) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hits++
	s.lastINM = r.Header.Get("If-None-Match")
	s.userAgent = r.Header.Get("User-Agent")
	if s.status != 0 {
		w.WriteHeader(s.status)
		return
	}
	if s.etag != "" && s.lastINM == s.etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("ETag", s.etag)
	_, _ = w.Write(s.body)
}

func (s *site) set(body []byte, etag string, status int) {
	s.mu.Lock()
	s.body, s.etag, s.status = body, etag, status
	s.mu.Unlock()
}

func (s *site) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.hits
}

func newTestStore(t *testing.T, url, cacheDir string) (*Store, *fakeClock) {
	t.Helper()
	clock := &fakeClock{t: time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)}
	s := NewStore(Options{URL: url, CacheDir: cacheDir, TTL: time.Hour, Timeout: 5 * time.Second, UserAgent: "biblical-atlas-mcp/test", Logf: t.Logf})
	s.now = clock.Now
	return s, clock
}

func startSite(t *testing.T, body []byte, etag string) (*site, string) {
	t.Helper()
	st := &site{body: body, etag: etag}
	srv := httptest.NewServer(st)
	t.Cleanup(srv.Close)
	return st, srv.URL + "/data.json"
}

func TestStoreRefreshCycle(t *testing.T) {
	ctx := context.Background()
	body := readFixture(t, "data.json")
	st, url := startSite(t, body, `"v1"`)
	s, clock := newTestStore(t, url, "")

	// First load.
	snap, err := s.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if snap.ETag != `"v1"` || snap.Generated != "2026-09-30" || st.count() != 1 || st.userAgent != "biblical-atlas-mcp/test" {
		t.Fatalf("first load: etag %q generated %q hits %d ua %q", snap.ETag, snap.Generated, st.count(), st.userAgent)
	}

	// Within the TTL nothing is fetched.
	clock.Add(30 * time.Minute)
	if again, _ := s.Snapshot(ctx); again != snap || st.count() != 1 {
		t.Fatal("fetched inside the TTL")
	}

	// 304 keeps the snapshot and moves checked_at.
	clock.Add(31 * time.Minute)
	if again, _ := s.Snapshot(ctx); again != snap {
		t.Fatal("stale snapshot not served at once")
	}
	s.waitIdle()
	if st.count() != 2 || st.lastINM != `"v1"` {
		t.Fatalf("conditional request: hits %d If-None-Match %q", st.count(), st.lastINM)
	}
	if again, _ := s.Snapshot(ctx); again != snap {
		t.Fatal("304 replaced the snapshot")
	}
	if got := s.Status(); !got.CheckedAt.Equal(clock.Now()) || got.Stale {
		t.Fatalf("status after 304: %+v", got)
	}

	// A changed ETag swaps the snapshot.
	st.set([]byte(strings.Replace(string(body), `"generado": "2026-09-30"`, `"generado": "2026-10-01"`, 1)), `"v2"`, 0)
	clock.Add(61 * time.Minute)
	s.Snapshot(ctx)
	s.waitIdle()
	fresh, _ := s.Snapshot(ctx)
	if fresh.Generated != "2026-10-01" || fresh.ETag != `"v2"` {
		t.Fatalf("new copy not published: %q %q", fresh.Generated, fresh.ETag)
	}

	// A 500 keeps the old copy and reports the error.
	st.set(nil, "", http.StatusInternalServerError)
	clock.Add(61 * time.Minute)
	s.Snapshot(ctx)
	s.waitIdle()
	kept, _ := s.Snapshot(ctx)
	status := s.Status()
	if kept != fresh || !status.Stale || !strings.Contains(status.RefreshError, "HTTP 500") {
		t.Fatalf("after 500: same=%v status=%+v", kept == fresh, status)
	}
	// It asks again only after another TTL.
	hits := st.count()
	s.Snapshot(ctx)
	s.waitIdle()
	if st.count() != hits {
		t.Fatal("retried before the TTL passed")
	}
	st.set(body, `"v3"`, 0)
	clock.Add(61 * time.Minute)
	s.Snapshot(ctx)
	s.waitIdle()
	if got := s.Status(); got.Stale || got.RefreshError != "" {
		t.Fatalf("status after recovery: %+v", got)
	}
}

// An old address that redirects every path to the new site still loads, and
// the ETag keeps working across the redirect.
func TestStoreFollowsRedirect(t *testing.T) {
	ctx := context.Background()
	body := readFixture(t, "data.json")
	st, newURL := startSite(t, body, `"v1"`)
	base := strings.TrimSuffix(newURL, "/data.json")
	var redirects int
	var mu sync.Mutex
	old := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		redirects++
		mu.Unlock()
		http.Redirect(w, r, base+r.URL.Path, http.StatusMovedPermanently)
	}))
	t.Cleanup(old.Close)
	s, clock := newTestStore(t, old.URL+"/data.json", "")

	snap, err := s.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if snap.ETag != `"v1"` || st.count() != 1 || st.userAgent != "biblical-atlas-mcp/test" {
		t.Fatalf("first load: etag %q hits %d ua %q", snap.ETag, st.count(), st.userAgent)
	}

	// The conditional request reaches the new site with the ETag and its 304 keeps the copy.
	clock.Add(61 * time.Minute)
	s.Snapshot(ctx)
	s.waitIdle()
	if st.count() != 2 || st.lastINM != `"v1"` {
		t.Fatalf("conditional request: hits %d If-None-Match %q", st.count(), st.lastINM)
	}
	if again, _ := s.Snapshot(ctx); again != snap {
		t.Fatal("304 after a redirect replaced the snapshot")
	}
	if got := s.Status(); got.Stale || got.RefreshError != "" {
		t.Fatalf("status after 304: %+v", got)
	}
	mu.Lock()
	defer mu.Unlock()
	if redirects != 2 {
		t.Fatalf("old address asked %d times, want 2", redirects)
	}
}

func TestStoreFirstLoadFailures(t *testing.T) {
	ctx := context.Background()
	valid := readFixture(t, "data.json")
	cases := []struct {
		name   string
		body   []byte
		status int
		cap    int64
		want   string
	}{
		{"truncated", valid[:len(valid)-200], 0, 0, "data file is not valid JSON"},
		{"html", []byte("<html><body>error</body></html>"), 0, 0, "data file is not a JSON object"},
		{"unknown format", readFixture(t, "unknown-format.json"), 0, 0,
			`unsupported data format "biblical-atlas/v9": this server reads biblical-atlas/v0. Upgrade biblical-atlas-mcp.`},
		{"too large", valid, 0, 1000, "data file is larger than 1000 bytes"},
		{"not found", nil, http.StatusNotFound, 0, "answered HTTP 404"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			st, url := startSite(t, c.body, `"x"`)
			st.status = c.status
			s, clock := newTestStore(t, url, "")
			if c.cap > 0 {
				s.maxBytes = c.cap
			}
			_, err := s.Snapshot(ctx)
			if err == nil || !strings.Contains(err.Error(), c.want) || !strings.Contains(err.Error(), url) {
				t.Fatalf("got %v, want %q and the URL", err, c.want)
			}
			// The next call inside the pause does not fetch again; after it, it does.
			s.Snapshot(ctx)
			if st.count() != 1 {
				t.Fatalf("retried inside the pause: %d hits", st.count())
			}
			clock.Add(RetryPause)
			s.Snapshot(ctx)
			if st.count() != 2 {
				t.Fatalf("did not retry after the pause: %d hits", st.count())
			}
		})
	}
}

func TestSizeErrorInMegabytes(t *testing.T) {
	s := NewStore(Options{})
	if got := s.sizeError().Error(); got != "data file is larger than 64 MB" {
		t.Fatalf("got %q", got)
	}
}

func TestDiskCache(t *testing.T) {
	ctx := context.Background()
	dir := filepath.Join(t.TempDir(), "cache")
	body := readFixture(t, "data.json")
	st, url := startSite(t, body, `"v1"`)

	a, _ := newTestStore(t, url, dir)
	if _, err := a.Snapshot(ctx); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"data.json", "meta.json"} {
		if _, err := os.Stat(filepath.Join(dir, f)); err != nil {
			t.Fatalf("cache file %s: %v", f, err)
		}
	}

	// A second store with the same URL starts from the cache without asking.
	st.set(nil, "", http.StatusInternalServerError)
	b, clock := newTestStore(t, url, dir)
	b.Start()
	b.waitIdle()
	snap, err := b.Snapshot(ctx)
	if err != nil || snap.ETag != `"v1"` || st.count() != 1 {
		t.Fatalf("cache start: err %v etag %q hits %d", err, snap.ETag, st.count())
	}
	// Once the cached copy is older than the TTL, it asks with the saved ETag.
	st.set(body, `"v1"`, 0)
	clock.Add(2 * time.Hour)
	b.Snapshot(ctx)
	b.waitIdle()
	if st.count() != 2 || st.lastINM != `"v1"` {
		t.Fatalf("revalidation: hits %d If-None-Match %q", st.count(), st.lastINM)
	}

	// A cache saved for another URL is ignored.
	other, url2 := startSite(t, body, `"w1"`)
	c, _ := newTestStore(t, url2, dir)
	snap, err = c.Snapshot(ctx)
	if err != nil || snap.ETag != `"w1"` || other.count() != 1 {
		t.Fatalf("other URL: err %v etag %q hits %d", err, snap.ETag, other.count())
	}
}

func TestUnwritableCacheStillServes(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, url := startSite(t, readFixture(t, "data.json"), `"v1"`)
	s, _ := newTestStore(t, url, filepath.Join(file, "cache"))
	if _, err := s.Snapshot(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestLocalFileNeverTouchesTheNetwork(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("local-file mode made a request: %s", r.URL)
	}))
	t.Cleanup(srv.Close)
	path := filepath.Join(t.TempDir(), "data.json")
	if err := os.WriteFile(path, readFixture(t, "data.json"), 0o644); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(t.TempDir(), "cache")
	clock := &fakeClock{t: time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)}
	s := NewStore(Options{URL: srv.URL, File: path, CacheDir: cache, TTL: time.Hour, Logf: t.Logf})
	s.now = clock.Now
	snap, err := s.Snapshot(context.Background())
	if err != nil || snap.Generated != "2026-09-30" || snap.Source != path {
		t.Fatalf("file load: %v", err)
	}
	// A changed file is reloaded after the TTL.
	changed := strings.Replace(string(readFixture(t, "data.json")), `"generado": "2026-09-30"`, `"generado": "2026-10-02"`, 1)
	if err := os.WriteFile(path, []byte(changed), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(time.Hour)
	if err := os.Chtimes(path, future, future); err != nil {
		t.Fatal(err)
	}
	clock.Add(2 * time.Hour)
	s.Snapshot(context.Background())
	s.waitIdle()
	snap, _ = s.Snapshot(context.Background())
	if snap.Generated != "2026-10-02" {
		t.Fatalf("file not reloaded: %q", snap.Generated)
	}
	if _, err := os.Stat(cache); !os.IsNotExist(err) {
		t.Fatal("local-file mode wrote a disk cache")
	}
}

func TestContextCancelWhileWaiting(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-block }))
	t.Cleanup(func() { close(block); srv.Close() })
	s, _ := newTestStore(t, srv.URL, "")
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := s.Snapshot(ctx); err == nil || !strings.Contains(err.Error(), "not available") {
		t.Fatalf("got %v", err)
	}
}

// TestStaleCacheServedWithoutWaiting: a disk copy older than the TTL is
// served at once while the download hangs.
func TestStaleCacheServedWithoutWaiting(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "cache")
	body := readFixture(t, "data.json")
	release := make(chan struct{})
	var mu sync.Mutex
	hang := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		h := hang
		mu.Unlock()
		if h {
			<-release
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write(body)
	}))
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	// Release the handler before closing the server, or a failed test hangs.
	t.Cleanup(func() { unblock(); srv.Close() })
	url := srv.URL + "/data.json"
	a, _ := newTestStore(t, url, dir)
	if _, err := a.Snapshot(context.Background()); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	hang = true
	mu.Unlock()

	b, clock := newTestStore(t, url, dir)
	clock.Add(2 * time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	start := time.Now()
	snap, err := b.Snapshot(ctx)
	if err != nil || snap == nil || snap.ETag != `"v1"` {
		t.Fatalf("stale cache not served: %v", err)
	}
	if took := time.Since(start); took > time.Second {
		t.Fatalf("waited %v for the download", took)
	}
	unblock()
	b.waitIdle()
}

func TestLocalFileTooLarge(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.json")
	if err := os.WriteFile(path, readFixture(t, "data.json"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := NewStore(Options{File: path})
	s.maxBytes = 1000
	if _, err := s.Snapshot(context.Background()); err == nil || !strings.Contains(err.Error(), "larger than 1000 bytes") {
		t.Fatalf("got %v", err)
	}
}

// TestBuildPanicKeepsTheOldCopy: a panic while indexing a new file is an
// error; the copy served before stays.
func TestBuildPanicKeepsTheOldCopy(t *testing.T) {
	ctx := context.Background()
	st, url := startSite(t, readFixture(t, "data.json"), `"v1"`)
	s, clock := newTestStore(t, url, "")
	first, err := s.Snapshot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	buildSnapshot = func(*File) *Snapshot { panic("boom") }
	t.Cleanup(func() { buildSnapshot = Build })
	st.set(readFixture(t, "data.json"), `"v2"`, 0)
	clock.Add(2 * time.Hour)
	s.Snapshot(ctx)
	s.waitIdle()
	kept, _ := s.Snapshot(ctx)
	status := s.Status()
	if kept != first || !status.Stale || !strings.Contains(status.RefreshError, "boom") {
		t.Fatalf("after a panic: same=%v status=%+v", kept == first, status)
	}
}
