package securitytests

import (
	"os"
	"strings"
	"testing"
)

func readSource(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestCertificatePathValidationIsPresent(t *testing.T) {
	source := readSource(t, "../web/entity/entity.go")
	for _, want := range []string{"isSafeAbsPath", "filepath.IsAbs", `strings.ContainsRune(path, '\x00')`, "filepath.Clean(path)"} {
		if !strings.Contains(source, want) {
			t.Fatalf("expected certificate path validation source to contain %q", want)
		}
	}
}

func TestXrayDownloadChecksumVerificationIsPresent(t *testing.T) {
	source := readSource(t, "../web/service/server.go")
	for _, want := range []string{"extractSHA256Digest", "sha256.New", "hex.EncodeToString", "checksum mismatch"} {
		if !strings.Contains(source, want) {
			t.Fatalf("expected xray download verification source to contain %q", want)
		}
	}
}

func TestLoginRateLimitAndSessionRotationArePresent(t *testing.T) {
	index := readSource(t, "../web/controller/index.go")
	session := readSource(t, "../web/session/session.go")
	for _, want := range []string{"recordLoginFailure", "clearLoginFailures", "login_locked"} {
		if !strings.Contains(index, want) {
			t.Fatalf("expected login controller source to contain %q", want)
		}
	}
	for _, want := range []string{"s.Clear()", "Options", "MaxAge: -1"} {
		if !strings.Contains(session, want) {
			t.Fatalf("expected session source to contain %q", want)
		}
	}
}
