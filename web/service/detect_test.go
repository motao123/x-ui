package service

import "testing"

func TestParseIPAPICom(t *testing.T) {
	result, err := parseIPAPICom([]byte(`{
		"status":"success",
		"countryCode":"US",
		"city":"Los Angeles",
		"isp":"Example ISP",
		"as":"AS123 Example",
		"mobile":false,
		"proxy":false,
		"hosting":true,
		"query":"203.0.113.1"
	}`))
	if err != nil {
		t.Fatalf("parse ip-api response: %v", err)
	}
	if result.IP != "203.0.113.1" || result.Country != "US" || result.IPType != "hosting" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestParseIPAPIIs(t *testing.T) {
	result, err := parseIPAPIIs([]byte(`{
		"ip":"203.0.113.2",
		"asn":{"asn":64500,"descr":"AS64500 TEST","org":"Example Org","type":"hosting","country":"US"},
		"company":{"name":"Example Company","type":"business"},
		"location":{"country_code":"JP","city":"Tokyo"},
		"is_mobile":false
	}`))
	if err != nil {
		t.Fatalf("parse ipapi.is response: %v", err)
	}
	if result.IP != "203.0.113.2" || result.Country != "JP" || result.Org != "Example Org" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestParseIPPure(t *testing.T) {
	result, err := parseIPPure([]byte(`{
		"ip":"203.0.113.3",
		"asn":64501,
		"asOrganization":"Residential ISP",
		"countryCode":"DE",
		"city":"Berlin",
		"isResidential":true
	}`))
	if err != nil {
		t.Fatalf("parse ippure response: %v", err)
	}
	if result.IP != "203.0.113.3" || result.ASN != "AS64501" || result.IPType != "isp" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestSelectDetectPlatforms(t *testing.T) {
	selected := selectDetectPlatforms([]string{"openai", "unknown", "apple", "openai"})
	if len(selected) != 2 {
		t.Fatalf("unexpected selected count: %d", len(selected))
	}
	if selected[0].id != "openai" || selected[1].id != "apple" {
		t.Fatalf("unexpected selected platforms: %#v", selected)
	}
}

func TestSelectDetectPlatformsFallback(t *testing.T) {
	selected := selectDetectPlatforms([]string{"unknown"})
	if len(selected) != len(detectPlatforms()) {
		t.Fatalf("expected fallback to all platforms, got %d", len(selected))
	}
}

func TestDetectCaches(t *testing.T) {
	setCachedIPResult(&DetectIPResult{IP: "203.0.113.4", CheckedAt: 1})
	if cached := getCachedIPResult(); cached != nil {
		t.Fatalf("expired IP cache should be ignored: %#v", cached)
	}

	setCachedIPResult(&DetectIPResult{IP: "203.0.113.5", CheckedAt: 4102444800})
	cached := getCachedIPResult()
	if cached == nil || cached.IP != "203.0.113.5" {
		t.Fatalf("expected IP cache hit, got %#v", cached)
	}

	setCachedUnlockResult("apple", DetectUnlockResult{Platform: "apple", Status: "unlocked", CheckedAt: 4102444800})
	unlock, ok := getCachedUnlockResult("apple")
	if !ok || unlock.Status != "unlocked" {
		t.Fatalf("expected unlock cache hit, got %#v ok=%v", unlock, ok)
	}
}

func TestExtractHelpers(t *testing.T) {
	if got := extractBetween("a Region:\"US\" z", "Region:\"", "\""); got != "US" {
		t.Fatalf("unexpected extractBetween result: %q", got)
	}
	if got := extractLineValue("a=b\nloc=US\n", "loc="); got != "US" {
		t.Fatalf("unexpected extractLineValue result: %q", got)
	}
	if got := firstNonEmpty("", "  value  "); got != "value" {
		t.Fatalf("unexpected firstNonEmpty result: %q", got)
	}
}
