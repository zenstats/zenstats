package uaparser

import (
	"strings"

	"github.com/ua-parser/uap-go/uaparser"
)

type UAParser struct {
	uaparser *uaparser.Parser
}

type Client struct {
	Os        *uaparser.Os
	UserAgent *uaparser.UserAgent
	Device    *uaparser.Device
	Screen    *ScreenSize
}

// save deivce type ex: Desktop Mobile Tablet
type ScreenSize struct {
	Family string
	Bot    bool
	Spider bool
}

func (ss *ScreenSize) ToString() string {
	return ss.Family
}

func (ss *ScreenSize) IsBot() bool {
	return ss.Bot
}

func (ss *ScreenSize) IsSpider() bool {
	return ss.Spider
}

func New() *UAParser {
	return &UAParser{
		uaparser: uaparser.NewFromSaved(),
	}
}

func (u *UAParser) Parse(ua string) *Client {

	client := u.uaparser.Parse(ua)

	device := u.getDeviceType(client, ua)
	return &Client{
		Device:    client.Device,
		Os:        client.Os,
		UserAgent: client.UserAgent,
		Screen: &ScreenSize{
			Family: device,
			Bot:    device == "Spider",
			Spider: device == "Spider",
		},
	}
}
func (u *UAParser) getDeviceType(client *uaparser.Client, rawUA string) string {
	device := client.Device.Family
	os := client.Os.Family
	ua := client.UserAgent.Family
	deviceLower := strings.ToLower(device)
	uaLower := strings.ToLower(ua)
	osLower := strings.ToLower(os)
	// The bundled uap-go rules do not yet recognize HarmonyOS.
	rawLower := strings.ToLower(rawUA)
	if osLower == "other" && strings.Contains(rawLower, "harmonyos") {
		osLower = "harmonyos"
	}

	// 优先判断爬虫（Spider），避免被其他设备类型覆盖
	if strings.Contains(deviceLower, "spider") ||
		strings.Contains(deviceLower, "bot") ||
		strings.Contains(deviceLower, "slurp") ||
		strings.Contains(deviceLower, "crawl") ||
		strings.Contains(uaLower, "spider") ||
		strings.Contains(uaLower, "bot") ||
		strings.Contains(uaLower, "slurp") ||
		strings.Contains(uaLower, "crawl") ||
		strings.Contains(uaLower, "robot") ||
		strings.Contains(uaLower, "scraper") {

		return "Spider"
	}

	if strings.Contains(deviceLower, "ipad") ||
		strings.Contains(deviceLower, "tablet") ||
		(osLower == "harmonyos" && strings.Contains(rawLower, "tablet")) ||
		(osLower == "android" && !strings.Contains(uaLower, "mobile")) {

		return "Tablet"
	}

	if deviceLower == "iphone" || deviceLower == "ipod" {

		return "Mobile"
	}

	// Check mobile OS families before desktop ones (Windows Phone contains Windows).
	mobileOS := []string{"harmonyos", "ios", "android", "windows phone", "kaios"}
	for _, osName := range mobileOS {
		if strings.Contains(osLower, osName) {
			return "Mobile"
		}
	}

	desktopOS := []string{
		"windows", "mac", "linux", "ubuntu", "fedora", "debian", "chrome os", "chromeos",
		"freebsd", "openbsd", "netbsd", "arch", "centos", "red hat", "suse", "mint",
		"manjaro", "nixos", "gentoo",
	}
	for _, osName := range desktopOS {
		if strings.Contains(osLower, osName) {
			return "Desktop"
		}
	}

	return "Other"
}
