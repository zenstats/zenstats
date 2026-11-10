package geoip

import (
	"github.com/oschwald/geoip2-golang"
	"path/filepath"
	"testing"
	"time"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("../../data", name))
	if err != nil {
		t.Fatal(err)
	}
	return path
}
func testGeoIP(t *testing.T) *GeoIP {
	t.Helper()
	g, err := newGeoIP(fixture(t, "GeoLite2-City.mmdb.test"), 100, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(g.Close)
	return g
}
func TestGeoIP(t *testing.T) {
	g := testGeoIP(t)
	data, err := g.GetCountryAndRegion("81.2.69.160")
	if err != nil || data.IsoCode != "GB" || data.Country == "" || data.City != "London" {
		t.Fatalf("lookup: %+v %v", data, err)
	}
	for range 2 {
		if _, err := g.GetCountryAndRegion("81.2.69.160"); err != nil {
			t.Fatal(err)
		}
	}
	if s := g.Status(); s.Lookups != 3 || s.CountryFound != 3 || s.CityFound != 3 {
		t.Fatalf("status: %+v", s)
	}
}
func TestGeoIPLanguageFallback(t *testing.T) {
	record := &geoip2.City{}
	record.Country.IsoCode = "US"
	record.Country.Names = map[string]string{"en": "United States"}
	record.City.Names = map[string]string{"en": "Example City"}
	record.Continent.Names = map[string]string{"en": "North America"}
	data := dataFromRecord(record)
	if data.Country != "United States" || data.City != "Example City" || data.Continent != "North America" {
		t.Fatal(data)
	}
	record.Country.Names = nil
	if dataFromRecord(record).Country != "US" {
		t.Fatal("ISO fallback missing")
	}
	if localizedName(map[string]string{"fr": "Paris", "de": "Paris DE"}) != "Paris DE" {
		t.Fatal("fallback must be deterministic")
	}
}
func TestGeoIPCountryOnly(t *testing.T) {
	g, err := newGeoIP(fixture(t, "GeoLite2-Country.mmdb.test"), 100, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close()
	data, err := g.GetCountryAndRegion("81.2.69.160")
	if err != nil || data.IsoCode != "GB" || data.City != "" || g.Status().CityAvailable {
		t.Fatalf("country-only: %+v %+v %v", data, g.Status(), err)
	}
}
