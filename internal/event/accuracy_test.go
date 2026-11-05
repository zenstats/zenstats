package event

import (
	"net/url"
	"testing"

	"github.com/zenstats/zenstats/internal/model"
	"github.com/zenstats/zenstats/internal/store/clickhouse/models"
)

func TestPagePathNormalization(t *testing.T) {
	for _, tc := range []struct {
		url, want string
		hash      bool
	}{
		{"https://example.com", "/", false},
		{"https://example.com/?utm_source=email", "/", false},
		{"https://example.com/#/pricing?token=secret", "/#/pricing", true},
		{"https://example.com/#/pricing?token=other", "/#/pricing", true},
		{"https://example.com/app#/pricing?plan=pro", "/app#/pricing", true},
		{"https://example.com/app#section", "/app", false},
	} {
		parsed, err := url.Parse(tc.url)
		if err != nil {
			t.Fatal(err)
		}
		if got := eventPathname(parsed, tc.hash); got != tc.want {
			t.Errorf("%s: got %q want %q", tc.url, got, tc.want)
		}
	}
}

func TestAcquisitionIgnoresInternalReferrersAndRetainsCampaigns(t *testing.T) {
	worker := &EventWork{}
	for _, tc := range []struct {
		page, referrer, source string
	}{
		{"https://example.com", "https://example.com/previous", ""},
		{"https://shop.example.co.uk", "https://WWW.EXAMPLE.CO.UK.:443/previous", ""},
		{"https://shop.example.com", "https://www.example.com/previous", ""},
		{"https://alice.github.io", "https://bob.github.io/previous", "bob.github.io"},
		{"https://example.com", "https://example.com.evil.test/previous", "example.com.evil.test"},
		{"https://example.com", "https://www.google.com/search?q=private", "www.google.com"},
		{"https://example.com", "javascript:alert(1)", ""},
	} {
		t.Run(tc.page+"_"+tc.referrer, func(t *testing.T) {
			event := &models.Events{}
			worker.PutSourceInfo(event, &model.EventRequest{URL: tc.page + "?utm_source=newsletter&utm_medium=email", Referrer: tc.referrer})
			if event.ReferrerSource != tc.source || event.UtmSource != "newsletter" || event.UtmMedium != "email" {
				t.Fatalf("incorrect attribution: %+v", event)
			}
			if tc.source == "" && event.Referrer != "" {
				t.Fatalf("internal referrer persisted: %q", event.Referrer)
			}
			worker.PutChannel(event)
			if event.Channel != "Email" {
				t.Fatalf("campaign lost: %q", event.Channel)
			}
		})
	}
}

func TestChannelMatchesDomainBoundaries(t *testing.T) {
	worker := &EventWork{}
	for _, tc := range []struct{ source, want string }{
		{"www.google.com", "Organic Search"}, {"google.co.uk", "Organic Search"},
		{"notgoogle.com", "Referral"}, {"google.com.evil.test", "Referral"},
		{"m.facebook.com", "Social"}, {"x.com", "Social"}, {"t.co", "Social"},
		{"facebook-news.test", "Referral"}, {"", "Direct"},
	} {
		event := &models.Events{ReferrerSource: tc.source}
		worker.PutChannel(event)
		if event.Channel != tc.want {
			t.Errorf("%q: got %q want %q", tc.source, event.Channel, tc.want)
		}
	}
}
