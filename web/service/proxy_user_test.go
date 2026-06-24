package service

import (
	"path/filepath"
	"strings"
	"testing"
	"x-ui/database"
)

func TestProxyUserSaveRejectsWeakToken(t *testing.T) {
	setupProxyUserTestDB(t)
	service := &ProxyUserService{}
	err := service.Save(&ProxyUserPayload{Name: "alice", Enable: true, Token: "short", UUID: "11111111-1111-4111-8111-111111111111", Password: "password"})
	if err == nil || !strings.Contains(err.Error(), "订阅令牌") {
		t.Fatalf("expected weak token validation error, got %v", err)
	}
}

func TestProxyUserSaveAcceptsStrongToken(t *testing.T) {
	setupProxyUserTestDB(t)
	service := &ProxyUserService{}
	err := service.Save(&ProxyUserPayload{Name: "alice", Enable: true, Token: "AbCdEfGhIjKlMnOpQrStUvWx", UUID: "11111111-1111-4111-8111-111111111111", Password: "password"})
	if err != nil {
		t.Fatalf("save strong token: %v", err)
	}
}

func setupProxyUserTestDB(t *testing.T) {
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
