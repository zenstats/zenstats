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

	device := u.getDeviceType(client)
	return &Client{
		Device:    client.Device,
		Os:        client.Os,
		UserAgent: client.UserAgent,
		Screen: &ScreenSize{
			Family: device,
			Bot:    device == "Spider" || device == "Other",
			Spider: device == "Spider",
		},
	}
}
func (u *UAParser) getDeviceType(client *uaparser.Client) string {
	device := client.Device.Family
	os := client.Os.Family
	ua := client.UserAgent.Family
	deviceLower := strings.ToLower(device)
	uaLower := strings.ToLower(ua)

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

	if strings.Contains(deviceLower, "other") {
		return "Other"
	}

	if strings.Contains(device, "iPad") ||
		strings.Contains(device, "Tablet") ||
		(os == "Android" && !strings.Contains(ua, "Mobile")) {

		return "Tablet"
	}

	if device == "iPhone" ||
		device == "iPod" ||
		(os == "Android" && strings.Contains(ua, "Mobile")) {

		return "Mobile"
	}

	return "Desktop"
}
