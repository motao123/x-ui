package controller

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"x-ui/database/model"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestReadConfiguredLogTailDisabled(t *testing.T) {
	t.Setenv("XUI_LOG_FILE", "")

	response, err := readConfiguredLogTail(0)
	if err != nil {
		t.Fatalf("read disabled log: %v", err)
	}
	if response.Enabled {
		t.Fatal("expected disabled response")
	}
}

func TestReadConfiguredLogTailSmallFile(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "x-ui.log")
	content := "first line\nsecond line\n"
	if err := os.WriteFile(logPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write log file: %v", err)
	}
	t.Setenv("XUI_LOG_FILE", logPath)

	response, err := readConfiguredLogTail(64 * 1024)
	if err != nil {
		t.Fatalf("read log tail: %v", err)
	}
	if !response.Enabled {
		t.Fatal("expected enabled response")
	}
	if response.FileName != "x-ui.log" {
		t.Fatalf("unexpected file name: %q", response.FileName)
	}
	if response.Content != content {
		t.Fatalf("unexpected content: %q", response.Content)
	}
	if response.Truncated {
		t.Fatal("small file should not be truncated")
	}
}

func TestReadConfiguredLogTailLargeFile(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "x-ui.log")
	content := strings.Repeat("a", 128) + "tail"
	if err := os.WriteFile(logPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write log file: %v", err)
	}
	t.Setenv("XUI_LOG_FILE", logPath)

	response, err := readConfiguredLogTail(4)
	if err != nil {
		t.Fatalf("read log tail: %v", err)
	}
	if response.Content != "tail" {
		t.Fatalf("unexpected tail content: %q", response.Content)
	}
	if !response.Truncated {
		t.Fatal("large file should be truncated")
	}
	if response.Offset != int64(len(content)-4) {
		t.Fatalf("unexpected offset: %d", response.Offset)
	}
}

func TestReadConfiguredLogTailRejectsDirectory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XUI_LOG_FILE", dir)

	_, err := readConfiguredLogTail(64 * 1024)
	if err == nil {
		t.Fatal("expected error for directory log path")
	}
}

func TestClampLogReadLimit(t *testing.T) {
	if got := clampLogReadLimit(0); got != defaultLogTailLimit {
		t.Fatalf("unexpected default limit: %d", got)
	}
	if got := clampLogReadLimit(maxLogReadLimit + 1); got != maxLogReadLimit {
		t.Fatalf("unexpected max limit: %d", got)
	}
	if got := clampLogReadLimit(123); got != 123 {
		t.Fatalf("unexpected custom limit: %d", got)
	}
}

func TestBuildDashboardResponse(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Inbound{}, &model.ProxyUser{}, &model.Certificate{}, &model.RouteRule{}, &model.Endpoint{}, &model.TrafficHistory{}); err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	if err := db.Create(&model.Inbound{Enable: true, Up: 100, Down: 200, Port: 10001, Tag: "in-1"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Inbound{Enable: false, Up: 10, Down: 20, Port: 10002, Tag: "in-2"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.ProxyUser{Name: "u1", Enable: true, Token: "t1"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.RouteRule{Name: "r1", Enable: true, OutboundTag: "direct"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Endpoint{Name: "e1", Enable: true, Type: "custom", Tag: "out"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.Certificate{Name: "c1", CertFile: "cert", KeyFile: "key"}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.TrafficHistory{Up: 110, Down: 220, RecordAt: now}).Error; err != nil {
		t.Fatal(err)
	}

	response, err := buildDashboardResponse(db)
	if err != nil {
		t.Fatal(err)
	}
	if response.Summary.Inbounds != 2 || response.Summary.EnabledInbound != 1 {
		t.Fatalf("unexpected inbound summary: %#v", response.Summary)
	}
	if response.Summary.TotalUp != 110 || response.Summary.TotalDown != 220 {
		t.Fatalf("unexpected traffic totals: %#v", response.Summary)
	}
	if response.Summary.EnabledUsers != 1 || response.Summary.EnabledRules != 1 || response.Summary.EnabledEndpoints != 1 || response.Summary.Certificates != 1 {
		t.Fatalf("unexpected business summary: %#v", response.Summary)
	}
	if len(response.History) != 1 {
		t.Fatalf("expected 1 history record, got %d", len(response.History))
	}
}
