package geoip

import (
	"errors"
	"log/slog"
	"net"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/oschwald/geoip2-golang"
	"github.com/oschwald/maxminddb-golang"
	"github.com/zenstats/zenstats/config"
)

type GeoIP struct {
	cache        *expirable.LRU[string, GeoData]
	geoDBPath    string
	geoDB        *geoip2.Reader
	countryDB    *geoip2.Reader
	ttl          time.Duration
	mu           sync.RWMutex
	updateMu     sync.Mutex
	lookups      atomic.Uint64
	countryFound atomic.Uint64
	cityFound    atomic.Uint64
	notPublic    atomic.Uint64
	invalid      atomic.Uint64
	notFound     atomic.Uint64
	lookupErrors atomic.Uint64
}

type GeoData struct {
	Country     string      `json:"country"`
	Continent   string      `json:"continent"`
	City        string      `json:"city"`
	IsoCode     string      `json:"iso_code"`
	Coordinates Coordinates `json:"coordinates"`
}
type Coordinates struct {
	Latitude  float64 `json:"latitude" ch:"latitude"`
	Longitude float64 `json:"longitude" ch:"longitude"`
}

var (
	instance *GeoIP
	once     sync.Once
)

func GetGeoIP() *GeoIP {
	once.Do(func() {
		var err error
		instance, err = newGeoIP(filepath.Join(config.Conf.DataPath, "GeoLite2-City.mmdb"), 10000, time.Hour)
		if err != nil {
			slog.Error("Failed to initialize GeoIP (will retry via cron)", "error", err)
		}
	})
	return instance
}

func newGeoIP(path string, cacheSize int, ttl time.Duration) (*GeoIP, error) {
	g := &GeoIP{cache: expirable.NewLRU[string, GeoData](cacheSize, nil, ttl), ttl: ttl, geoDBPath: path}
	fallback := filepath.Join(filepath.Dir(path), "GeoLite2-City-fallback.mmdb")
	if db, err := openDatabase(path); err == nil {
		g.geoDB = db
		if other, err := openDatabase(fallback); err == nil {
			g.countryDB = other
		}
		slog.Info("GeoIP database loaded", "type", db.Metadata().DatabaseType, "built_at", time.Unix(int64(db.Metadata().BuildEpoch), 0))
		return g, nil
	}
	if db, err := openDatabase(fallback); err == nil {
		g.geoDB = db
		slog.Warn("Using country-level fallback GeoIP database; city data may be unavailable", "type", db.Metadata().DatabaseType)
		return g, nil
	}
	// Keep the initialized instance even on failure, so a later update can recover.
	if err := g.UpdateGeoIPDB(""); err != nil {
		return g, err
	}
	return g, nil
}

func openDatabase(path string) (*geoip2.Reader, error) {
	check, err := maxminddb.Open(path)
	if err != nil {
		return nil, err
	}
	err = check.Verify()
	_ = check.Close()
	if err != nil {
		return nil, err
	}
	db, err := geoip2.Open(path)
	if err != nil {
		if db != nil {
			_ = db.Close()
		}
		return nil, err
	}
	// Validate both the database type and search tree before replacing a live DB.
	_, err = db.City(net.ParseIP("8.8.8.8"))
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func (g *GeoIP) GetCountryAndRegion(ip string) (GeoData, error) {
	g.lookups.Add(1)
	if ip == "" {
		g.invalid.Add(1)
		return GeoData{}, errors.New("IP address is empty")
	}
	addr := net.ParseIP(strings.TrimSpace(ip))
	if addr == nil {
		g.invalid.Add(1)
		return GeoData{}, nil
	}
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() {
		g.notPublic.Add(1)
		return GeoData{}, nil
	}
	key := addr.String()
	// Hold the reader through the lookup AND cache insertion: an update must not
	// close an active mmap or allow an old result to repopulate the purged cache.
	g.mu.RLock()
	defer g.mu.RUnlock()
	if g.geoDB == nil {
		g.lookupErrors.Add(1)
		return GeoData{}, errors.New("GeoIP database not initialized")
	}
	if data, ok := g.cache.Get(key); ok {
		g.countResult(data)
		return data, nil
	}
	record, err := g.geoDB.City(addr)
	if err != nil {
		g.lookupErrors.Add(1)
		return GeoData{}, err
	}
	data := dataFromRecord(record)
	if data.IsoCode == "" && g.countryDB != nil {
		extra, err := g.countryDB.City(addr)
		if err != nil {
			g.lookupErrors.Add(1)
		} else {
			fallback := dataFromRecord(extra)
			data.Country, data.IsoCode = fallback.Country, fallback.IsoCode
			if data.Continent == "" {
				data.Continent = fallback.Continent
			}
		}
	}
	g.cache.Add(key, data)
	g.countResult(data)
	return data, nil
}

func dataFromRecord(record *geoip2.City) GeoData {
	code := strings.ToUpper(record.Country.IsoCode)
	country := localizedName(record.Country.Names)
	if !validCountryCode(code) {
		code = ""
		country = ""
	}
	if country == "" {
		country = code
	}
	return GeoData{Country: country, IsoCode: code, Continent: localizedName(record.Continent.Names), City: localizedName(record.City.Names), Coordinates: Coordinates{Latitude: record.Location.Latitude, Longitude: record.Location.Longitude}}
}
func validCountryCode(code string) bool {
	return len(code) == 2 && code[0] >= 'A' && code[0] <= 'Z' && code[1] >= 'A' && code[1] <= 'Z'
}
func localizedName(names map[string]string) string {
	for _, lang := range []string{"zh-CN", "en"} {
		if name := strings.TrimSpace(names[lang]); name != "" {
			return name
		}
	}
	keys := make([]string, 0, len(names))
	for key := range names {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if name := strings.TrimSpace(names[key]); name != "" {
			return name
		}
	}
	return ""
}
func (g *GeoIP) countResult(data GeoData) {
	if data.IsoCode != "" {
		g.countryFound.Add(1)
	} else {
		g.notFound.Add(1)
	}
	if data.City != "" {
		g.cityFound.Add(1)
	}
}

// Status contains no IPs, credentials or local paths. Counters include cache hits
// and all callers (including heatmap shielding), not deduplicated visitors.
type Status struct {
	Ready                  bool       `json:"ready"`
	DatabaseType           string     `json:"database_type"`
	BuiltAt                *time.Time `json:"built_at"`
	CityAvailable          bool       `json:"city_available"`
	Stale                  bool       `json:"stale"`
	CountryFallbackBuiltAt *time.Time `json:"country_fallback_built_at"`
	Lookups                uint64     `json:"lookups"`
	CountryFound           uint64     `json:"country_found"`
	CityFound              uint64     `json:"city_found"`
	NotPublic              uint64     `json:"not_public"`
	Invalid                uint64     `json:"invalid"`
	NotFound               uint64     `json:"not_found"`
	Errors                 uint64     `json:"errors"`
}

func (g *GeoIP) Status() Status {
	g.mu.RLock()
	defer g.mu.RUnlock()
	s := Status{Ready: g.geoDB != nil, Lookups: g.lookups.Load(), CountryFound: g.countryFound.Load(), CityFound: g.cityFound.Load(), NotPublic: g.notPublic.Load(), Invalid: g.invalid.Load(), NotFound: g.notFound.Load(), Errors: g.lookupErrors.Load()}
	if g.geoDB != nil {
		m := g.geoDB.Metadata()
		at := time.Unix(int64(m.BuildEpoch), 0).UTC()
		s.BuiltAt = &at
		s.DatabaseType = m.DatabaseType
		s.CityAvailable = cityDatabase(m.DatabaseType)
		s.Stale = time.Since(at) > 45*24*time.Hour
	}
	if g.countryDB != nil {
		at := time.Unix(int64(g.countryDB.Metadata().BuildEpoch), 0).UTC()
		s.CountryFallbackBuiltAt = &at
	}
	return s
}
func cityDatabase(kind string) bool {
	return strings.Contains(kind, "City") || strings.Contains(kind, "Enterprise") || strings.Contains(kind, "Location")
}

// Close is used for non-singleton readers such as isolated tests and commands.
func (g *GeoIP) Close() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.geoDB != nil {
		_ = g.geoDB.Close()
		g.geoDB = nil
	}
	if g.countryDB != nil {
		_ = g.countryDB.Close()
		g.countryDB = nil
	}
}
