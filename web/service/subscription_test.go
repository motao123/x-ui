package service

import (
	"encoding/base64"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"x-ui/database"
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
	if strings.Contains(link, "pinPeerCertSha256") {
		t.Fatalf("vless link should not include pinPeerCertSha256 when unset: %s", link)
	}
}

func TestBuildSubscriptionVLESSLinkEmitsPinnedPeerCert(t *testing.T) {
	user := &model.ProxyUser{Name: "alice", UUID: "11111111-1111-4111-8111-111111111111"}
	inbound := &model.Inbound{
		Remark:         "tls",
		Port:           443,
		Protocol:       model.VLESS,
		Settings:       `{"clients":[{"id":"old"}],"decryption":"none"}`,
		StreamSettings: `{"network":"tcp","security":"tls","tlsSettings":{"serverName":"example.com","pinnedPeerCertSha256":"abcdef0123456789"}}`,
	}
	link := buildSubscriptionLink(user, inbound, "example.com")
	if !strings.Contains(link, "pinPeerCertSha256=abcdef0123456789") {
		t.Fatalf("vless link missing pinPeerCertSha256: %s", link)
	}
}

func TestBuildSubscriptionTrojanLinkEmitsPinnedPeerCert(t *testing.T) {
	user := &model.ProxyUser{Name: "alice", Password: "p@ss"}
	inbound := &model.Inbound{
		Remark:         "trojan",
		Port:           443,
		Protocol:       model.Trojan,
		Settings:       `{"clients":[{"password":"p@ss"}]}`,
		StreamSettings: `{"network":"tcp","security":"tls","tlsSettings":{"serverName":"example.com","pinnedPeerCertSha256":"abcdef0123456789"}}`,
	}
	link := buildSubscriptionLink(user, inbound, "example.com")
	if !strings.HasPrefix(link, "trojan://") {
		t.Fatalf("unexpected trojan link: %s", link)
	}
	if !strings.Contains(link, "pinPeerCertSha256=abcdef0123456789") {
		t.Fatalf("trojan link missing pinPeerCertSha256: %s", link)
	}
}

func TestBuildSubscriptionLinkDoesNotUseRequestHostFallback(t *testing.T) {
	user := &model.ProxyUser{Name: "alice", UUID: "11111111-1111-4111-8111-111111111111"}
	inbound := &model.Inbound{Remark: "reality", Port: 443, Protocol: model.VLESS, StreamSettings: `{"network":"tcp","security":"reality"}`}

	if link := buildSubscriptionLink(user, inbound, ""); link != "" {
		t.Fatalf("expected empty link without configured host or inbound listen, got %s", link)
	}
	inbound.Listen = "203.0.113.10"
	link := buildSubscriptionLink(user, inbound, "attacker.example")
	if !strings.Contains(link, "@203.0.113.10:443") || strings.Contains(link, "attacker.example") {
		t.Fatalf("link should prefer inbound listen over request host: %s", link)
	}
}

func TestRecordAccessDeduplicatesAndPrunes(t *testing.T) {
	setupSubscriptionTestDB(t)
	service := &SubscriptionService{}
	old := time.Now().Add(-subscriptionAccessRetention - time.Hour).Unix()
	if err := database.GetDB().Create(&model.SubscriptionAccess{ProxyUserId: 2, Format: "raw", UserAgent: "old", RemoteIp: "127.0.0.1", AccessedAt: old}).Error; err != nil {
		t.Fatal(err)
	}

	service.recordAccess(1, "raw", "ua", "198.51.100.1")
	service.recordAccess(1, "raw", "ua", "198.51.100.1")

	var count int64
	if err := database.GetDB().Model(&model.SubscriptionAccess{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected one deduplicated recent access after pruning, got %d", count)
	}
}

func setupSubscriptionTestDB(t *testing.T) {
	t.Helper()
	if err := database.InitDB(filepath.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if sqlDB, err := database.GetDB().DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
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
