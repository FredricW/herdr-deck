package update

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// DefaultMaxAge is how long a remote check stays good: every deck shares
// the cache, so the source is asked at most once an hour.
const DefaultMaxAge = time.Hour

// Checker is the deck's "update available" check. It asks herdr how the
// deck is installed, then reuses the last remote answer while it is fresh,
// so only one deck an hour runs git ls-remote. It never changes anything
// but its cache file.
type Checker struct {
	Updater Updater
	// CachePath is the shared cache file; "" keeps no cache.
	CachePath string
	// MaxAge is how long a remote answer is reused; zero is DefaultMaxAge.
	MaxAge time.Duration
	Now    func() time.Time
}

// cacheFile is the cache's JSON.
type cacheFile struct {
	Source    string    `json:"source"`
	CheckedAt time.Time `json:"checked_at"`
	Remote    Remote    `json:"remote"`
}

// Check reports whether something newer exists. Failures (offline, no
// herdr) come back as errors for the Sources view.
func (c Checker) Check(ctx context.Context) (Status, error) {
	in, err := c.Updater.Install(ctx)
	if err != nil {
		return Status{}, err
	}
	now := time.Now()
	if c.Now != nil {
		now = c.Now()
	}
	maxAge := c.MaxAge
	if maxAge == 0 {
		maxAge = DefaultMaxAge
	}
	r, ok := c.read(in.source(), now, maxAge)
	if !ok {
		if r, err = c.Updater.Latest(ctx, in); err != nil {
			return Status{Install: in}, err
		}
		c.write(cacheFile{Source: in.source(), CheckedAt: now, Remote: r})
	}
	return c.Updater.Compare(ctx, in, r), nil
}

// read returns the cached answer for source when it is younger than maxAge.
// A cache from the future (a clock change) is stale.
func (c Checker) read(source string, now time.Time, maxAge time.Duration) (Remote, bool) {
	if c.CachePath == "" {
		return Remote{}, false
	}
	b, err := os.ReadFile(c.CachePath)
	if err != nil {
		return Remote{}, false
	}
	var f cacheFile
	if json.Unmarshal(b, &f) != nil || f.Source != source {
		return Remote{}, false
	}
	if age := now.Sub(f.CheckedAt); age < 0 || age >= maxAge {
		return Remote{}, false
	}
	return f.Remote, true
}

// write saves f through a rename, so a deck never reads half a file. A
// cache that cannot be written only costs another check.
func (c Checker) write(f cacheFile) {
	if c.CachePath == "" {
		return
	}
	b, err := json.Marshal(f)
	if err != nil {
		return
	}
	dir := filepath.Dir(c.CachePath)
	if os.MkdirAll(dir, 0o755) != nil {
		return
	}
	tmp, err := os.CreateTemp(dir, ".update-*.json")
	if err != nil {
		return
	}
	_, werr := tmp.Write(b)
	cerr := tmp.Close()
	if werr != nil || cerr != nil || os.Rename(tmp.Name(), c.CachePath) != nil {
		os.Remove(tmp.Name())
	}
}
