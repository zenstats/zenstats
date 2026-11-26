package uaparser

import (
	"strings"
	"testing"

	"github.com/ua-parser/uap-go/uaparser"
)

func TestUAParser_Parse(t *testing.T) {
	parser := New()
	tests := []struct {
		name, ua, family string
		bot              bool
	}{
		{"Ubuntu", "Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/109.0", "Desktop", false},
		{"Fedora", "Mozilla/5.0 (X11; Fedora; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/109.0", "Desktop", false},
		{"Debian", "Mozilla/5.0 (X11; Linux x86_64; rv:109.0) Gecko/20100101 Firefox/109.0 Debian/12", "Desktop", false},
		{"Chrome OS", "Mozilla/5.0 (X11; CrOS x86_64 14541.0.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Safari/537.36", "Desktop", false},
		{"FreeBSD", "Mozilla/5.0 (X11; FreeBSD amd64; rv:109.0) Gecko/20100101 Firefox/109.0", "Desktop", false},
		{"HarmonyOS", "Mozilla/5.0 (Phone; HarmonyOS 4.0; HUAWEI ALN-AL00) AppleWebKit/537.36 (KHTML, like Gecko) HuaweiBrowser/14.0.0.300 Mobile Safari/537.36", "Mobile", false},
		{"HarmonyOS Tablet", "Mozilla/5.0 (Tablet; HarmonyOS 4.0; HUAWEI DBY-W09) AppleWebKit/537.36 (KHTML, like Gecko) HuaweiBrowser/14.0.0.300 Safari/537.36", "Tablet", false},
		{"HarmonyOS Googlebot", "Mozilla/5.0 (Phone; HarmonyOS 4.0) (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", "Spider", true},
		{"iPhone", "Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1", "Mobile", false},
		{"iPad", "Mozilla/5.0 (iPad; CPU OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1", "Tablet", false},
		{"Android Mobile", "Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Mobile Safari/537.36", "Mobile", false},
		{"Android Tablet", "Mozilla/5.0 (Linux; Android 13; SM-X700) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Safari/537.36", "Tablet", false},
		{"Windows", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Safari/537.36", "Desktop", false},
		{"macOS", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15", "Desktop", false},
		{"Googlebot", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", "Spider", true},
		{"Googlebot Android", "Mozilla/5.0 (Linux; Android 6.0.1; Nexus 5X Build/MMB29P) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/110.0.0.0 Mobile Safari/537.36 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", "Spider", true},
		{"Unknown", "unrecognized-user-agent/1.0", "Other", false},
		{"okhttp", "okhttp/4.9.1", "Other", false},
		{"Empty", "", "Other", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := parser.Parse(tt.ua)
			if client.Screen.Family != tt.family || client.Screen.ToString() != tt.family {
				t.Errorf("family = %q, want %q (OS=%q, device=%q, browser=%q)", client.Screen.Family, tt.family, client.Os.Family, client.Device.Family, client.UserAgent.Family)
			}
			if client.Screen.IsBot() != tt.bot || client.Screen.IsSpider() != tt.bot {
				t.Errorf("bot=%t spider=%t, want both %t", client.Screen.IsBot(), client.Screen.IsSpider(), tt.bot)
			}
		})
	}
}

func TestGetDeviceTypeOSFamilies(t *testing.T) {
	parser := &UAParser{}
	classify := func(os, device, browser string) string {
		return parser.getDeviceType(&uaparser.Client{
			Os: &uaparser.Os{Family: os}, Device: &uaparser.Device{Family: device},
			UserAgent: &uaparser.UserAgent{Family: browser},
		}, "")
	}
	for _, os := range []string{
		"Windows", "Mac OS X", "Linux", "Ubuntu", "Fedora", "Debian", "Chrome OS", "ChromeOS",
		"FreeBSD", "OpenBSD", "NetBSD", "Arch", "CentOS", "Red Hat", "SUSE", "Mint", "Manjaro", "NixOS", "Gentoo",
	} {
		t.Run(os, func(t *testing.T) {
			for _, variant := range []string{os, strings.ToLower(os), strings.ToUpper(os)} {
				if got := classify(variant, "Other", "Firefox"); got != "Desktop" {
					t.Errorf("OS %q: got %q, want Desktop", variant, got)
				}
			}
		})
	}
	for _, os := range []string{"HarmonyOS", "iOS", "Android", "Windows Phone", "KaiOS"} {
		t.Run(os, func(t *testing.T) {
			for _, variant := range []string{os, strings.ToLower(os), strings.ToUpper(os)} {
				if got := classify(variant, "Other", "Chrome Mobile"); got != "Mobile" {
					t.Errorf("OS %q: got %q, want Mobile", variant, got)
				}
				for _, device := range []string{"iPad", "Tablet"} {
					if got := classify(variant, device, "Chrome Mobile"); got != "Tablet" {
						t.Errorf("OS %q device %q: got %q, want Tablet", variant, device, got)
					}
				}
				if got := classify(variant, "Spider", "Chrome Mobile"); got != "Spider" {
					t.Errorf("OS %q: Spider classified as %q", variant, got)
				}
			}
		})
	}
}
