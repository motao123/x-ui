package service

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type testHTTPRequest struct {
	URL *url.URL
}

type testHTTPResponse struct {
	StatusCode int
	Body       string
}

type testRoundTripper struct {
	handler func(*testHTTPRequest) testHTTPResponse
}

func (t testRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	response := t.handler(&testHTTPRequest{URL: req.URL})
	return &http.Response{
		StatusCode: response.StatusCode,
		Body:       io.NopCloser(strings.NewReader(response.Body)),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

func newTestHTTPClient(handler func(*testHTTPRequest) testHTTPResponse) *http.Client {
	return &http.Client{Transport: testRoundTripper{handler: handler}}
}

func TestFilterXrayVersions(t *testing.T) {
	releases := []Release{
		{TagName: "v25.10.15"},
		{TagName: "v25.10.15"},
		{TagName: "bad"},
		{TagName: "v26.0.0", Draft: true},
		{TagName: "v26.1.0", Prerelease: true},
		{TagName: " v25.9.11 "},
	}

	versions := filterXrayVersions(releases)
	if strings.Join(versions, ",") != "v25.10.15,v25.9.11" {
		t.Fatalf("unexpected versions: %#v", versions)
	}
}

func TestExtractSHA256DigestMatchesAssetBasename(t *testing.T) {
	digest := strings.Repeat("a", 64)
	text := "SHA2-256 (Xray-linux-64.zip) = " + digest

	if got := extractSHA256Digest(text, "Xray-linux-64.zip"); got != digest {
		t.Fatalf("unexpected digest: %q", got)
	}
	if got := extractSHA256Digest(text, filepath.Join(t.TempDir(), "Xray-linux-64.zip")); got != "" {
		t.Fatalf("full local path should not match digest line directly: %q", got)
	}
}

func TestVerifyXrayDigestUsesAssetBasename(t *testing.T) {
	oldClient := httpClient
	defer func() { httpClient = oldClient }()

	dir := t.TempDir()
	zipPath := filepath.Join(dir, "Xray-linux-64.zip")
	body := []byte("xray package")
	if err := os.WriteFile(zipPath, body, 0o600); err != nil {
		t.Fatalf("write zip: %v", err)
	}
	sum := sha256.Sum256(body)
	digest := hex.EncodeToString(sum[:])
	var requestedPath string
	httpClient = newTestHTTPClient(func(req *testHTTPRequest) testHTTPResponse {
		requestedPath = req.URL.Path
		return testHTTPResponse{StatusCode: 200, Body: "SHA2-256 (Xray-linux-64.zip) = " + digest}
	})

	if err := verifyXrayDigest("v25.10.15", zipPath); err != nil {
		t.Fatalf("verify digest: %v", err)
	}
	if !strings.HasSuffix(requestedPath, "/v25.10.15/Xray-linux-64.zip.dgst") {
		t.Fatalf("digest request used wrong path: %s", requestedPath)
	}
}

func TestVerifyXrayDigestRejectsMismatch(t *testing.T) {
	oldClient := httpClient
	defer func() { httpClient = oldClient }()

	zipPath := filepath.Join(t.TempDir(), "Xray-linux-64.zip")
	if err := os.WriteFile(zipPath, []byte("actual"), 0o600); err != nil {
		t.Fatalf("write zip: %v", err)
	}
	httpClient = newTestHTTPClient(func(req *testHTTPRequest) testHTTPResponse {
		return testHTTPResponse{StatusCode: 200, Body: "SHA2-256 (Xray-linux-64.zip) = " + strings.Repeat("b", 64)}
	})

	if err := verifyXrayDigest("v25.10.15", zipPath); err == nil {
		t.Fatal("expected checksum mismatch")
	}
}

func TestPrepareXrayPackageFilesDoesNotModifyTargetsOnMissingFile(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "xray.zip")
	createZip(t, zipPath, map[string]string{
		"xray":        "new-binary",
		"geosite.dat": "new-geosite",
	})

	reader := openZipReader(t, zipPath)
	targets := []xrayPackageFile{
		{zipName: "xray", path: filepath.Join(dir, "xray"), perm: 0o755},
		{zipName: "geosite.dat", path: filepath.Join(dir, "geosite.dat"), perm: 0o644},
		{zipName: "geoip.dat", path: filepath.Join(dir, "geoip.dat"), perm: 0o644},
	}
	for _, target := range targets {
		if err := os.WriteFile(target.path, []byte("old-"+target.zipName), 0o600); err != nil {
			t.Fatalf("write target: %v", err)
		}
	}

	if _, err := prepareXrayPackageFiles(reader, targets); err == nil {
		t.Fatal("expected missing geoip error")
	}
	for _, target := range targets {
		data, err := os.ReadFile(target.path)
		if err != nil {
			t.Fatalf("read target: %v", err)
		}
		if !strings.HasPrefix(string(data), "old-") {
			t.Fatalf("target was modified before complete prepare: %s=%q", target.path, data)
		}
	}
}

func TestReplaceAndRollbackXrayPackageFiles(t *testing.T) {
	dir := t.TempDir()
	targets := []xrayPackageFile{
		{zipName: "xray", path: filepath.Join(dir, "xray"), perm: 0o755},
		{zipName: "geosite.dat", path: filepath.Join(dir, "geosite.dat"), perm: 0o644},
	}
	temps := map[string]string{}
	for _, target := range targets {
		if err := os.WriteFile(target.path, []byte("old-"+target.zipName), 0o600); err != nil {
			t.Fatalf("write target: %v", err)
		}
		tmpFile, err := os.CreateTemp(dir, "new-*.tmp")
		if err != nil {
			t.Fatalf("create temp: %v", err)
		}
		if _, err := io.WriteString(tmpFile, "new-"+target.zipName); err != nil {
			t.Fatalf("write temp: %v", err)
		}
		tmpFile.Close()
		temps[target.path] = tmpFile.Name()
	}

	backups, err := replaceXrayPackageFiles(targets, temps)
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(targets[0].path)
		if err != nil {
			t.Fatalf("stat binary: %v", err)
		}
		if info.Mode().Perm() != 0o755 {
			t.Fatalf("unexpected binary mode: %v", info.Mode().Perm())
		}
	}
	if err := rollbackXrayPackageFiles(targets, backups); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	for _, target := range targets {
		data, err := os.ReadFile(target.path)
		if err != nil {
			t.Fatalf("read target: %v", err)
		}
		if !strings.HasPrefix(string(data), "old-") {
			t.Fatalf("rollback did not restore %s: %q", target.path, data)
		}
	}
}

func createZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("create zip: %v", err)
	}
	writer := zip.NewWriter(file)
	for name, content := range files {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create zip entry: %v", err)
		}
		if _, err := io.WriteString(entry, content); err != nil {
			t.Fatalf("write zip entry: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}
}

func openZipReader(t *testing.T, path string) *zip.Reader {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	t.Cleanup(func() { file.Close() })
	stat, err := file.Stat()
	if err != nil {
		t.Fatalf("stat zip: %v", err)
	}
	reader, err := zip.NewReader(file, stat.Size())
	if err != nil {
		t.Fatalf("new zip reader: %v", err)
	}
	return reader
}
