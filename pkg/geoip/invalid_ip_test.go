package geoip

import (
	"testing"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
)

// TestGetCountryAndRegionEmptyIPReturnsError verifies early rejection of blank input.
func TestGetCountryAndRegionEmptyIPReturnsError(t *testing.T) {
	g := &GeoIP{}
	_, err := g.GetCountryAndRegion("")
	if err == nil {
		t.Fatal("expected an error for empty IP address, got nil")
	}
}

// TestGetCountryAndRegionInvalidIPReturnsEmptyData verifies the net.ParseIP nil
// check fix: an invalid IP string must not panic or reach db.City(nil).
// When the GeoIP DB is available the function returns (GeoData{}, nil).
// When it is not available (nil geoDB) the function returns an error before
// reaching the ParseIP path — so we test both scenarios.
func TestGetCountryAndRegionInvalidIPReturnsEmptyData(t *testing.T) {
	g := testGeoIP(t)
	if g.geoDB == nil {
		t.Skip("GeoIP database not available — skipping invalid-IP path test")
	}

	invalidIPs := []string{
		"not-an-ip",
		"999.999.999.999",
		"::gggg",
		"hostname.example.com",
		"256.1.1.1",
	}
	for _, ip := range invalidIPs {
		t.Run(ip, func(t *testing.T) {
			data, err := g.GetCountryAndRegion(ip)
			if err != nil {
				t.Fatalf("expected nil error for invalid IP %q, got: %v", ip, err)
			}
			if data != (GeoData{}) {
				t.Fatalf("expected empty GeoData for invalid IP %q, got: %+v", ip, data)
			}
		})
	}
}

// TestGetCountryAndRegionNilDBReturnsError verifies behavior when the GeoIP
// database has not been initialized (geoDB == nil).
func TestGetCountryAndRegionNilDBReturnsError(t *testing.T) {
	g := &GeoIP{
		cache: expirable.NewLRU[string, GeoData](100, nil, time.Minute),
	}
	_, err := g.GetCountryAndRegion("8.8.8.8")
	if err == nil {
		t.Fatal("expected an error when geoDB is nil, got nil")
	}
}

func TestGeoIPNonPublicAddresses(t *testing.T) {
	g := testGeoIP(t)
	for _, ip := range []string{"127.0.0.1", "10.1.2.3", "192.168.1.1", "::1", "fd00::1", "fe80::1", "0.0.0.0", "224.0.0.1"} {
		data, err := g.GetCountryAndRegion(ip)
		if err != nil || data != (GeoData{}) {
			t.Fatalf("nonpublic %s: %+v %v", ip, data, err)
		}
	}
	if g.Status().NotPublic != 8 {
		t.Fatal(g.Status())
	}
}
