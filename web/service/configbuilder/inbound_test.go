package configbuilder

import (
	"strings"
	"testing"
	"x-ui/database/model"
)

func TestNormalizedSettingsRemovesFlowWithoutXTLSOrReality(t *testing.T) {
	settings := NormalizedSettings(
		model.VLESS,
		`{"clients":[{"id":"uuid","flow":"xtls-rprx-direct"}],"decryption":"none"}`,
		`{"network":"tcp","security":"none"}`,
	)
	if strings.Contains(settings, "flow") {
		t.Fatalf("flow should be removed from non-XTLS/non-Reality settings: %s", settings)
	}
}

func TestNormalizedSettingsRemovesTrojanFlowWithoutXTLSOrReality(t *testing.T) {
	settings := NormalizedSettings(
		model.Trojan,
		`{"clients":[{"password":"pass","flow":"xtls-rprx-direct"}]}`,
		`{"network":"tcp","security":"tls"}`,
	)
	if strings.Contains(settings, "flow") {
		t.Fatalf("flow should be removed from Trojan TLS settings: %s", settings)
	}
}

func TestNormalizedSettingsKeepsXTLSFlow(t *testing.T) {
	settings := NormalizedSettings(
		model.Trojan,
		`{"clients":[{"password":"pass","flow":"xtls-rprx-vision"}]}`,
		`{"network":"tcp","security":"xtls"}`,
	)
	if !strings.Contains(settings, "xtls-rprx-vision") {
		t.Fatalf("XTLS flow should be preserved: %s", settings)
	}
}

func TestNormalizedSettingsKeepsRealityFlow(t *testing.T) {
	settings := NormalizedSettings(
		model.VLESS,
		`{"clients":[{"id":"uuid","flow":"xtls-rprx-vision"}],"decryption":"none"}`,
		`{"network":"tcp","security":"reality"}`,
	)
	if !strings.Contains(settings, "xtls-rprx-vision") {
		t.Fatalf("reality flow should be preserved: %s", settings)
	}
}
