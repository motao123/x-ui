package configbuilder

import (
	"strings"
	"testing"
	"x-ui/database/model"
)

func TestBuildInboundConfigWithProxyUsersOverridesVLESSClients(t *testing.T) {
	inbound := &model.Inbound{
		Port:           443,
		Protocol:       model.VLESS,
		Settings:       `{"clients":[{"id":"old","flow":"xtls-rprx-vision"}],"decryption":"none"}`,
		StreamSettings: `{"network":"tcp","security":"reality"}`,
		Tag:            "inbound-443",
	}
	cfg := BuildInboundConfigWithUsers(inbound, []*model.ProxyUser{{Name: "alice", UUID: "new-id"}})
	settings := string(cfg.Settings)
	if !strings.Contains(settings, "new-id") || strings.Contains(settings, "old") || !strings.Contains(settings, "xtls-rprx-vision") {
		t.Fatalf("proxy user clients were not injected: %s", settings)
	}
}
