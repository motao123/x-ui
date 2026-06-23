package service

import (
	"encoding/base64"
	"strings"
	"testing"
	"x-ui/database/model"
)

func TestBuildSubscriptionVLESSLink(t *testing.T) {
	user := &model.ProxyUser{Name: "alice", UUID: "11111111-1111-4111-8111-111111111111"}
	inbound := &model.Inbound{
		Remark:         "reality",
		Port:           443,
		Protocol:       model.VLESS,
		Settings:       `{"clients":[{"id":"old","flow":"xtls-rprx-vision"}],"decryption":"none"}`,
		StreamSettings: `{"network":"tcp","security":"reality"}`,
	}
	link := buildSubscriptionLink(user, inbound, "example.com")
	if !strings.HasPrefix(link, "vless://11111111-1111-4111-8111-111111111111@example.com:443") {
		t.Fatalf("unexpected vless link: %s", link)
	}
	if !strings.Contains(link, "security=reality") || !strings.Contains(link, "flow=xtls-rprx-vision") {
		t.Fatalf("vless link missing query fields: %s", link)
	}
}

func TestBase64SubscriptionEncoding(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte("vless://example"))
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != "vless://example" {
		t.Fatalf("unexpected decoded subscription: %s", decoded)
	}
}
