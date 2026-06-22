package controller

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
