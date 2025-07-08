package uaparser

import (
	"testing"
)

func TestUAParser_Parse(t *testing.T) {
	ua := New()

	uas := []string{
		"okhttp/4.9.1",
	}

	for _, u := range uas {
		client := ua.Parse(u)
		t.Logf("UserAgent: %s\n", u)
		t.Logf("OS: %s\n", client.Os.Family)
		t.Logf("OS Version: %s\n", client.Os.ToVersionString())
		t.Logf("Browser: %s\n", client.UserAgent.Family)
		t.Logf("Browser Version: %s\n", client.UserAgent.ToVersionString())
		t.Logf("Device: %s\n", client.Device.ToString())
		t.Logf("Screen: %s\n", client.Screen.Family)
		t.Logf("ISbot: %t\n", client.Screen.IsBot())
		t.Logf("Spider: %t\n", client.Screen.IsSpider())
	}
}
