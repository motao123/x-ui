package model

import (
	"strings"
	"testing"
)

func TestNormalizedSettingsRemovesFlowWithoutXTLSOrReality(t *testing.T) {
	inbound := &Inbound{
		Protocol:       VLESS,
		Settings:       `{"clients":[{"id":"uuid","flow":"xtls-rprx-direct"}],"decryption":"none"}`,
		StreamSettings: `{"network":"tcp","security":"none"}`,
	}

	settings := inbound.normalizedSettings(inbound.StreamSettings)
	if settings == inbound.Settings {
		t.Fatal("expected settings to be normalized")
	}
	if strings.Contains(settings, "flow") {
		t.Fatalf("flow should be removed from non-XTLS/non-Reality settings: %s", settings)
	}
}

func TestNormalizedSettingsKeepsRealityFlow(t *testing.T) {
	inbound := &Inbound{
		Protocol:       VLESS,
		Settings:       `{"clients":[{"id":"uuid","flow":"xtls-rprx-vision"}],"decryption":"none"}`,
		StreamSettings: `{"network":"tcp","security":"reality"}`,
	}

	settings := inbound.normalizedSettings(inbound.StreamSettings)
	if !strings.Contains(settings, "xtls-rprx-vision") {
		t.Fatalf("reality flow should be preserved: %s", settings)
	}
}
