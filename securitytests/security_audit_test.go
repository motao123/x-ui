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

func TestAcmeCertificatesAreReadableByXrayUser(t *testing.T) {
	acme := readSource(t, "../web/controller/acme.go")
	permissions := readSource(t, "../web/controller/acme_permissions_unix.go")
	install := readSource(t, "../install.sh")
	for _, want := range []string{"setAcmeCertificatePermissions", "0640", "filepath.Dir(acmeBaseDir)"} {
		if !strings.Contains(acme, want) {
			t.Fatalf("expected ACME source to contain %q", want)
		}
	}
	for _, want := range []string{"user.LookupGroup(\"xray\")", "os.Chown", "0750", "0640"} {
		if !strings.Contains(permissions, want) {
			t.Fatalf("expected ACME permissions source to contain %q", want)
		}
	}
	for _, want := range []string{"chown :xray /etc/x-ui", "chmod 0750 /etc/x-ui"} {
		if !strings.Contains(install, want) {
			t.Fatalf("expected install script to contain %q", want)
		}
	}
}

func TestReleasePackageIncludesXrayGeodata(t *testing.T) {
	workflow := readSource(t, "../.github/workflows/release.yml")
	for _, want := range []string{"cp bin/*.dat release/amd64/x-ui/bin/", "cp bin/*.dat release/arm64/x-ui/bin/", "hashFiles('bin/*.dat'"} {
		if !strings.Contains(workflow, want) {
			t.Fatalf("expected release workflow to contain %q", want)
		}
	}
}
