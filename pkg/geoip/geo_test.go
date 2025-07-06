package geoip

import (
	"fmt"
	"testing"
)

func TestGeoIP(t *testing.T) {
	// Initialize GeoIP
	geoip := GetGeoIP()

	// Test GeoIP.GetCountryAndRegion
	ip := "114.114.114.114"
	geoData, err := geoip.GetCountryAndRegion(ip)
	if err != nil {
		t.Fatalf("Failed to get country and region for IP %s: %v", ip, err)
	}
	fmt.Println(geoData)
	if geoData.Country != "中国" || geoData.Continent != "亚洲" || geoData.IsoCode != "CN" {
		t.Errorf("Unexpected result for IP %s: %+v", ip, geoData)
	}
	ip = "8.8.8.8"
	geoData, err = geoip.GetCountryAndRegion(ip)
	if err != nil {
		t.Fatalf("Failed to get country and region for IP %s: %v", ip, err)
	}
	if geoData.Country != "美国" || geoData.Continent != "北美洲" || geoData.IsoCode != "US" {
		t.Errorf("Unexpected result for IP %s: %+v", ip, geoData)
	}
	ip = "125.92.206.12"
	geoData, err = geoip.GetCountryAndRegion(ip)
	fmt.Println(geoData)
	if err != nil {
		t.Fatalf("Failed to get country and region for IP %s: %v", ip, err)
	}
	if geoData.Country != "美国" || geoData.Continent != "北美洲" || geoData.IsoCode != "US" {
		t.Errorf("Unexpected result for IP %s: %+v", ip, geoData)
	}
}
