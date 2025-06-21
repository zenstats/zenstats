package bootstrap

import (
	"github.com/zenstats/zenstats/pkg/geoip"
)

func InitGeoIP() {
	geoip.GetGeoIP()
}
