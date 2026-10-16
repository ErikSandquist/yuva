package analytics

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/oschwald/maxminddb-golang/v2"
)

const (
	GeoDBIP = "dbip"
	GeoFile = "file"
	GeoOff  = "off"

	DBIPAttribution    = "IP Geolocation by DB-IP"
	DBIPAttributionURL = "https://db-ip.com"
	dbipPrefix         = "dbip-city-lite-"
	maxGeoIPBytes      = 1 << 30
)

// DBIPURL is where DB-IP publishes the month's IP to City Lite database (CC BY 4.0).
func DBIPURL(month time.Time) string {
	return "https://download.db-ip.com/free/" + dbipPrefix + month.Format("2006-01") + ".mmdb.gz"
}

// Location is where a shortened address is, as far as the database knows.
type Location struct {
	Country, Region, City string
}

type GeoInfo struct {
	Source  string
	BuiltAt time.Time
	Error   string
}

// GeoIP looks up shortened addresses in an MMDB file: with dbip, the month's DB-IP Lite release that
// Update downloads into Dir; with file, an MMDB the operator keeps at File; with off, nothing.
type GeoIP struct {
	Mode   string
	Dir    string
	File   string
	Client *http.Client
	URL    func(month time.Time) string
	Now    func() time.Time

	mu      sync.RWMutex
	db      *maxminddb.Reader
	path    string
	modTime time.Time
	lastErr string
	update  sync.Mutex
}

type geoRecord struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
	Subdivisions []struct {
		Names struct {
			En string `maxminddb:"en"`
		} `maxminddb:"names"`
	} `maxminddb:"subdivisions"`
	City struct {
		Names struct {
			En string `maxminddb:"en"`
		} `maxminddb:"names"`
	} `maxminddb:"city"`
}

// Lookup takes only a shortened address, so the full one never reaches the database.
func (g *GeoIP) Lookup(ip ShortIP) Location {
	if g == nil {
		return Location{}
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.db == nil {
		return Location{}
	}
	var rec geoRecord
	if err := g.db.Lookup(ip.Addr()).Decode(&rec); err != nil {
		return Location{}
	}
	loc := Location{Country: strings.ToUpper(rec.Country.ISOCode), City: Clean(rec.City.Names.En, maxFieldLength)}
	if len(loc.Country) != 2 {
		loc.Country = ""
	}
	if len(rec.Subdivisions) > 0 {
		loc.Region = Clean(rec.Subdivisions[0].Names.En, maxFieldLength)
	}
	return loc
}

func (g *GeoIP) Info() GeoInfo {
	if g == nil {
		return GeoInfo{Source: GeoOff}
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	out := GeoInfo{Source: g.Mode, Error: g.lastErr}
	if g.db != nil {
		out.BuiltAt = g.db.Metadata.BuildTime().UTC()
	}
	return out
}

func (g *GeoIP) setErr(err error) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.lastErr = ""
	if err != nil {
		g.lastErr = err.Error()
	}
	return err
}

// Load opens the newest database there is, if it is not open already.
func (g *GeoIP) Load() error {
	if g == nil || g.Mode == GeoOff {
		return nil
	}
	path := g.File
	if g.Mode == GeoDBIP {
		path = g.newest()
		if path == "" {
			return g.setErr(errors.New("no DB-IP database downloaded yet"))
		}
	}
	st, err := os.Stat(path)
	if err != nil {
		return g.setErr(err)
	}
	g.mu.RLock()
	same := g.db != nil && g.path == path && g.modTime.Equal(st.ModTime())
	g.mu.RUnlock()
	if same {
		return g.setErr(nil)
	}
	db, err := maxminddb.Open(path)
	if err != nil {
		return g.setErr(fmt.Errorf("open %s: %w", filepath.Base(path), err))
	}
	g.mu.Lock()
	old := g.db
	g.db, g.path, g.modTime, g.lastErr = db, path, st.ModTime(), ""
	g.mu.Unlock()
	if old != nil {
		_ = old.Close()
	}
	return nil
}

func (g *GeoIP) newest() string {
	entries, err := os.ReadDir(g.Dir)
	if err != nil {
		return ""
	}
	var names []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), dbipPrefix) && strings.HasSuffix(e.Name(), ".mmdb") {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		return ""
	}
	return filepath.Join(g.Dir, slices.Max(names))
}

// Update makes the newest database the open one: with dbip it downloads this month's release
// (last month's while this one is not out yet) unless it is already there; with file it reopens
// the file when it changed. It reports the file in use.
func (g *GeoIP) Update(ctx context.Context) (string, error) {
	if g == nil || g.Mode == GeoOff {
		return "", nil
	}
	g.update.Lock()
	defer g.update.Unlock()
	if g.Mode == GeoDBIP {
		if err := g.download(ctx); err != nil {
			_ = g.Load()
			return "", g.setErr(err)
		}
	}
	if err := g.Load(); err != nil {
		return "", err
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.path, nil
}

func (g *GeoIP) download(ctx context.Context) error {
	now := time.Now
	if g.Now != nil {
		now = g.Now
	}
	url := DBIPURL
	if g.URL != nil {
		url = g.URL
	}
	if err := os.MkdirAll(g.Dir, 0o755); err != nil {
		return err
	}
	t := now().UTC()
	month := time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
	for _, m := range []time.Time{month, month.AddDate(0, -1, 0)} {
		name := filepath.Join(g.Dir, dbipPrefix+m.Format("2006-01")+".mmdb")
		if _, err := os.Stat(name); err == nil {
			g.prune(name)
			return nil
		}
		err := g.fetch(ctx, url(m), name)
		if errors.Is(err, errNotPublished) {
			continue
		}
		if err != nil {
			return err
		}
		g.prune(name)
		return nil
	}
	return errors.New("DB-IP has published no database for this month or the last")
}

var errNotPublished = errors.New("not published")

func (g *GeoIP) fetch(ctx context.Context, url, name string) error {
	client := g.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		return errNotPublished
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, res.Status)
	}
	zr, err := gzip.NewReader(res.Body)
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	tmp, err := os.CreateTemp(g.Dir, ".dbip-*.mmdb")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	n, err := io.Copy(tmp, io.LimitReader(zr, maxGeoIPBytes+1))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return fmt.Errorf("download %s: %w", url, err)
	}
	if n > maxGeoIPBytes {
		return fmt.Errorf("download %s: larger than %d bytes", url, maxGeoIPBytes)
	}
	db, err := maxminddb.Open(tmp.Name())
	if err != nil {
		return fmt.Errorf("download %s: not an MMDB file: %w", url, err)
	}
	_ = db.Close()
	return os.Rename(tmp.Name(), name)
}

// prune deletes the releases older than keep, and leftovers of downloads that did not finish.
func (g *GeoIP) prune(keep string) {
	entries, err := os.ReadDir(g.Dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		n := e.Name()
		path := filepath.Join(g.Dir, n)
		if path == keep {
			continue
		}
		old := strings.HasPrefix(n, dbipPrefix) && strings.HasSuffix(n, ".mmdb") && n < filepath.Base(keep)
		if info, err := e.Info(); err == nil && strings.HasPrefix(n, ".dbip-") && time.Since(info.ModTime()) > time.Hour {
			old = true
		}
		if old {
			_ = os.Remove(path)
		}
	}
}

func (g *GeoIP) Close() error {
	if g == nil {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.db == nil {
		return nil
	}
	err := g.db.Close()
	g.db = nil
	return err
}
