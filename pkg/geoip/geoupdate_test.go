package geoip

import (
	"testing"
)

func TestUpdateGeoIPDB(t *testing.T) {

	geoip := GetGeoIP()
	err := geoip.UpdateGeoIPDB("")
	if err != nil {
		t.Errorf("UpdateGeoIPDB() failed with error: %v", err)
	}
}
