package logger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/op/go-logging"
)

func TestInitLoggerWritesToConfiguredFile(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "x-ui.log")
	t.Setenv("XUI_LOG_FILE", logPath)
	t.Setenv("XUI_LOG_MAX_SIZE_MB", "1")
	t.Setenv("XUI_LOG_MAX_BACKUPS", "1")
	t.Setenv("XUI_LOG_MAX_AGE_DAYS", "1")

	InitLogger(logging.INFO)
	t.Cleanup(func() {
		t.Setenv("XUI_LOG_FILE", "")
		InitLogger(logging.INFO)
	})
	Info("file logging enabled")

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !strings.Contains(string(data), "file logging enabled") {
		t.Fatalf("log file did not contain written message: %q", string(data))
	}
}

func TestInitLoggerUsesDefaultsWithoutFile(t *testing.T) {
	t.Setenv("XUI_LOG_FILE", "")

	InitLogger(logging.INFO)
	Info("stderr logging enabled")
}
