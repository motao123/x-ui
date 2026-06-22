package xray

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestAtomicWriteFileWritesAndReplacesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	if err := atomicWriteFile(path, []byte(`{"version":1}`), 0o600); err != nil {
		t.Fatalf("initial write failed: %v", err)
	}
	if err := atomicWriteFile(path, []byte(`{"version":2}`), 0o600); err != nil {
		t.Fatalf("replacement write failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read written file: %v", err)
	}
	if string(data) != `{"version":2}` {
		t.Fatalf("unexpected file contents: %q", string(data))
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat written file: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("unexpected file permissions: %v", info.Mode().Perm())
	}
}

func TestAtomicWriteFileCleansTempFileOnRenameFailure(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "target")
	if err := os.Mkdir(targetDir, 0o700); err != nil {
		t.Fatalf("create target directory: %v", err)
	}

	err := atomicWriteFile(targetDir, []byte("data"), 0o600)
	if err == nil {
		t.Fatal("expected rename failure")
	}

	matches, err := filepath.Glob(filepath.Join(dir, "*.tmp"))
	if err != nil {
		t.Fatalf("glob temp files: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files were not cleaned up: %v", matches)
	}
}
