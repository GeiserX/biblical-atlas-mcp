package atlas

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// cacheMeta is meta.json beside the cached data.json.
type cacheMeta struct {
	URL       string    `json:"url"`
	ETag      string    `json:"etag"`
	FetchedAt time.Time `json:"fetched_at"`
}

// loadCache publishes the disk copy when it was saved for the same URL and
// still validates. Anything wrong with it is ignored; the next good fetch
// overwrites it.
func (s *Store) loadCache() {
	dir := s.opt.CacheDir
	raw, err := os.ReadFile(filepath.Join(dir, "meta.json"))
	if err != nil {
		return
	}
	var meta cacheMeta
	if json.Unmarshal(raw, &meta) != nil || meta.URL != s.opt.URL {
		return
	}
	body, err := os.ReadFile(filepath.Join(dir, "data.json"))
	if err != nil || int64(len(body)) > s.maxBytes {
		return
	}
	sn, err := s.build(body, meta.ETag, s.opt.URL, meta.FetchedAt)
	if err != nil {
		s.opt.Logf("ignoring the disk cache: %v", err)
		return
	}
	s.publish(sn)
	s.mu.Lock()
	s.checkedAt = meta.FetchedAt
	s.nextCheck = meta.FetchedAt.Add(s.opt.TTL)
	s.mu.Unlock()
}

// writeCache saves the body and its meta. A failure is logged and the
// server keeps running from memory.
func (s *Store) writeCache(body []byte, etag string, at time.Time) {
	dir := s.opt.CacheDir
	if dir == "" {
		return
	}
	meta, _ := json.Marshal(cacheMeta{URL: s.opt.URL, ETag: etag, FetchedAt: at})
	if err := os.MkdirAll(dir, 0o755); err != nil {
		s.opt.Logf("disk cache disabled: %v", err)
		return
	}
	for _, f := range []struct {
		name string
		data []byte
	}{{"data.json", body}, {"meta.json", meta}} {
		if err := writeAtomic(filepath.Join(dir, f.name), f.data); err != nil {
			s.opt.Logf("disk cache not written: %v", err)
			return
		}
	}
}

func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}
