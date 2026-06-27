package geoip

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/oschwald/geoip2-golang"
	"github.com/zenstats/zenstats/config"
	"golang.org/x/exp/slog"
)

type GeoIP struct {
	cache     *expirable.LRU[string, GeoData]
	geoDBPath string
	geoDB     *geoip2.Reader
	ttl       time.Duration
	mu        sync.RWMutex
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
	instance  *GeoIP
	once      sync.Once
	defaultDB string
)

func GetGeoIP() *GeoIP {
	once.Do(func() {
		defaultDB = filepath.Join(config.Conf.DataPath, "GeoLite2-City.mmdb")

		geoIP, err := newGeoIP(defaultDB, 10000, 1*time.Hour)
		if err != nil {
			slog.Error("Failed to initialize GeoIP (will retry via cron)", "error", err)
			// 创建一个空实例，允许服务启动，GeoIP 数据库将通过定时任务下载
			instance = &GeoIP{
				cache: expirable.NewLRU[string, GeoData](10000, nil, 1*time.Hour),
				ttl:   1 * time.Hour,
			}
			return
		}
		instance = geoIP
	})
	return instance
}

func newGeoIP(geoipDBPath string, cacheSize int, ttl time.Duration) (*GeoIP, error) {
	l := expirable.NewLRU[string, GeoData](cacheSize, nil, ttl)
	geoip := &GeoIP{
		cache: l,
		ttl:   ttl,
	}

	if _, err := os.Stat(geoipDBPath); os.IsNotExist(err) {
		fallbackPath := filepath.Join(filepath.Dir(geoipDBPath), "GeoLite2-City-fallback.mmdb")
		if _, fallbackErr := os.Stat(fallbackPath); fallbackErr == nil {
			geoipDBPath = fallbackPath
			slog.Info("Using fallback GeoIP database", "path", fallbackPath)
		} else {
			var err error
			geoipDBPath, err = geoip.DownloadGeoIPDB()
			if err != nil {
				return nil, err
			}
		}
	}

	geoipDB, err := geoip2.Open(geoipDBPath)
	if err != nil {
		return nil, err
	}
	geoip.geoDB = geoipDB

	return geoip, nil
}

func (g *GeoIP) GetCountryAndRegion(ip string) (GeoData, error) {
	if ip == "" {
		return GeoData{}, errors.New("IP address is empty")
	}

	// Check cache first
	if val, ok := g.cache.Get(ip); ok {
		return val, nil
	}

	g.mu.RLock()
	db := g.geoDB
	g.mu.RUnlock()

	// 如果 GeoIP 数据库未初始化，返回空数据
	if db == nil {
		return GeoData{}, errors.New("GeoIP database not initialized")
	}

	// Lookup IP in GeoIP database
	ipAddr := net.ParseIP(ip)
	if ipAddr == nil {
		return GeoData{}, nil
	}
	record, err := db.City(ipAddr)
	if err != nil {
		return GeoData{}, err
	}

	geoData := GeoData{
		Country:   record.Country.Names["zh-CN"],
		Continent: record.Continent.Names["zh-CN"],
		City:      record.City.Names["zh-CN"],
		IsoCode:   record.Country.IsoCode,
		Coordinates: Coordinates{
			Latitude:  record.Location.Latitude,
			Longitude: record.Location.Longitude,
		},
	}

	// Cache the result
	g.cache.Add(ip, geoData)

	return geoData, nil
}
