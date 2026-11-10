package geoip

import (
	"archive/tar"
	"compress/gzip"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/zenstats/zenstats/config"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestUpdateGeoIPDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GeoLite2-City.mmdb")
	g := &GeoIP{geoDBPath: path, cache: expirable.NewLRU[string, GeoData](100, nil, time.Minute)}
	defer g.Close()
	if err := g.UpdateGeoIPDB(fixture(t, "GeoLite2-City.mmdb.test")); err != nil {
		t.Fatal(err)
	}
	g.cache.Add("81.2.69.160", GeoData{})
	if err := g.UpdateGeoIPDB(fixture(t, "GeoLite2-City.mmdb.test")); err != nil {
		t.Fatal(err)
	}
	data, err := g.GetCountryAndRegion("81.2.69.160")
	if err != nil || data.IsoCode != "GB" {
		t.Fatal(data, err)
	}
	bad := filepath.Join(t.TempDir(), "corrupt.mmdb")
	os.WriteFile(bad, []byte("download error page"), 0600)
	if err := g.UpdateGeoIPDB(bad); err == nil {
		t.Fatal("corrupt update accepted")
	}
	data, err = g.GetCountryAndRegion("81.2.69.160")
	if err != nil || data.IsoCode != "GB" {
		t.Fatal("valid reader lost after failed update", data, err)
	}
	// Country updates supplement rather than downgrade an installed City DB.
	if err := g.UpdateGeoIPDB(fixture(t, "GeoLite2-Country.mmdb.test")); err != nil {
		t.Fatal(err)
	}
	data, err = g.GetCountryAndRegion("81.2.69.160")
	if err != nil || data.City != "London" || !g.Status().CityAvailable || g.Status().CountryFallbackBuiltAt == nil {
		t.Fatal(data, g.Status(), err)
	}
	// Persistence survives a restart, including the supplemental country reader.
	restarted, err := newGeoIP(path, 100, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	if restarted.Status().CountryFallbackBuiltAt == nil {
		t.Fatal("fallback not reopened")
	}
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 100 {
				if _, err := g.GetCountryAndRegion("81.2.69.160"); err != nil {
					t.Error(err)
				}
			}
		}()
	}
	for range 4 {
		if err := g.UpdateGeoIPDB(fixture(t, "GeoLite2-City.mmdb.test")); err != nil {
			t.Fatal(err)
		}
	}
	wg.Wait()
}
func TestExtractGeoIPArchive(t *testing.T) {
	original := config.Conf
	copyConfig := *original
	copyConfig.DataPath = t.TempDir()
	config.Conf = &copyConfig
	defer func() { config.Conf = original }()
	path := filepath.Join(t.TempDir(), "db.tar.gz")
	makeArchive := func(name string, body []byte) {
		t.Helper()
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		gz := gzip.NewWriter(f)
		tw := tar.NewWriter(gz)
		tw.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: int64(len(body))})
		tw.Write(body)
		tw.Close()
		gz.Close()
		f.Close()
	}
	makeArchive("README", []byte("no database"))
	g := &GeoIP{}
	if _, err := g.extractGzFile(path); err == nil {
		t.Fatal("archive without DB accepted")
	}
	contents, err := os.ReadFile(fixture(t, "GeoLite2-City.mmdb.test"))
	if err != nil {
		t.Fatal(err)
	}
	makeArchive("untrusted/path/GeoLite2-City.mmdb", contents)
	candidate, err := g.extractGzFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(candidate)
	db, err := openDatabase(candidate)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	if filepath.Dir(candidate) != copyConfig.DataPath {
		t.Fatal("archive path escaped candidate directory")
	}
}
