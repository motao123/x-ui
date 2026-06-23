package service

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
	"x-ui/logger"
	"x-ui/util/sys"
	"x-ui/xray"

	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"
)

type ProcessState string

const (
	Running ProcessState = "running"
	Stop    ProcessState = "stop"
	Error   ProcessState = "error"
)

type Status struct {
	T   time.Time `json:"-"`
	Cpu float64   `json:"cpu"`
	Mem struct {
		Current uint64 `json:"current"`
		Total   uint64 `json:"total"`
	} `json:"mem"`
	Swap struct {
		Current uint64 `json:"current"`
		Total   uint64 `json:"total"`
	} `json:"swap"`
	Disk struct {
		Current uint64 `json:"current"`
		Total   uint64 `json:"total"`
	} `json:"disk"`
	Xray struct {
		State    ProcessState `json:"state"`
		ErrorMsg string       `json:"errorMsg"`
		Version  string       `json:"version"`
	} `json:"xray"`
	Uptime   uint64    `json:"uptime"`
	Loads    []float64 `json:"loads"`
	TcpCount int       `json:"tcpCount"`
	UdpCount int       `json:"udpCount"`
	NetIO    struct {
		Up   uint64 `json:"up"`
		Down uint64 `json:"down"`
	} `json:"netIO"`
	NetTraffic struct {
		Sent uint64 `json:"sent"`
		Recv uint64 `json:"recv"`
	} `json:"netTraffic"`
}

type Release struct {
	TagName    string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

type ServerService struct {
	xrayService XrayService
}

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &userAgentTransport{
		inner: http.DefaultTransport,
	},
}

type userAgentTransport struct {
	inner http.RoundTripper
}

func (t *userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", "x-ui/1.0")
	}
	return t.inner.RoundTrip(req)
}

var xrayVersionPattern = regexp.MustCompile(`^v\d+\.\d+\.\d+([-.][0-9A-Za-z.-]+)?$`)
var sha256HexPattern = regexp.MustCompile(`(?i)\b[a-f0-9]{64}\b`)

const (
	maxXrayDownloadSize = 200 * 1024 * 1024
	maxXrayFileSize     = 150 * 1024 * 1024
)

func (s *ServerService) GetStatus(lastStatus *Status) *Status {
	now := time.Now()
	status := &Status{
		T: now,
	}

	percents, err := cpu.Percent(0, false)
	if err != nil {
		logger.Warning("get cpu percent failed:", err)
	} else {
		status.Cpu = percents[0]
	}

	upTime, err := host.Uptime()
	if err != nil {
		logger.Warning("get uptime failed:", err)
	} else {
		status.Uptime = upTime
	}

	memInfo, err := mem.VirtualMemory()
	if err != nil {
		logger.Warning("get virtual memory failed:", err)
	} else {
		status.Mem.Current = memInfo.Used
		status.Mem.Total = memInfo.Total
	}

	swapInfo, err := mem.SwapMemory()
	if err != nil {
		logger.Warning("get swap memory failed:", err)
	} else {
		status.Swap.Current = swapInfo.Used
		status.Swap.Total = swapInfo.Total
	}

	distInfo, err := disk.Usage("/")
	if err != nil {
		logger.Warning("get dist usage failed:", err)
	} else {
		status.Disk.Current = distInfo.Used
		status.Disk.Total = distInfo.Total
	}

	avgState, err := load.Avg()
	if err != nil {
		logger.Warning("get load avg failed:", err)
	} else {
		status.Loads = []float64{avgState.Load1, avgState.Load5, avgState.Load15}
	}

	ioStats, err := net.IOCounters(false)
	if err != nil {
		logger.Warning("get io counters failed:", err)
	} else if len(ioStats) > 0 {
		ioStat := ioStats[0]
		status.NetTraffic.Sent = ioStat.BytesSent
		status.NetTraffic.Recv = ioStat.BytesRecv

		if lastStatus != nil {
			duration := now.Sub(lastStatus.T)
			seconds := float64(duration) / float64(time.Second)
			up := uint64(float64(status.NetTraffic.Sent-lastStatus.NetTraffic.Sent) / seconds)
			down := uint64(float64(status.NetTraffic.Recv-lastStatus.NetTraffic.Recv) / seconds)
			status.NetIO.Up = up
			status.NetIO.Down = down
		}
	} else {
		logger.Warning("can not find io counters")
	}

	status.TcpCount, err = sys.GetTCPCount()
	if err != nil {
		logger.Warning("get tcp connections failed:", err)
	}

	status.UdpCount, err = sys.GetUDPCount()
	if err != nil {
		logger.Warning("get udp connections failed:", err)
	}

	if s.xrayService.IsXrayRunning() {
		status.Xray.State = Running
		status.Xray.ErrorMsg = ""
	} else {
		err := s.xrayService.GetXrayErr()
		if err != nil {
			status.Xray.State = Error
		} else {
			status.Xray.State = Stop
		}
		status.Xray.ErrorMsg = s.xrayService.GetXrayResult()
	}
	status.Xray.Version = s.xrayService.GetXrayVersion()

	return status
}

var fallbackXrayVersions = []string{"v25.10.15", "v25.9.11", "v25.8.31"}

func (s *ServerService) GetXrayVersions() ([]string, error) {
	url := "https://api.github.com/repos/XTLS/Xray-core/releases"
	resp, err := httpClient.Get(url)
	if err != nil {
		logger.Warning("get xray versions failed, using fallback versions:", err)
		return fallbackXrayVersions, nil
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		logger.Warning("get xray versions returned status", resp.StatusCode, ", using fallback versions")
		return fallbackXrayVersions, nil
	}
	buffer := bytes.NewBuffer(make([]byte, 8192))
	buffer.Reset()
	_, err = buffer.ReadFrom(resp.Body)
	if err != nil {
		logger.Warning("read xray versions failed, using fallback versions:", err)
		return fallbackXrayVersions, nil
	}

	releases := make([]Release, 0)
	err = json.Unmarshal(buffer.Bytes(), &releases)
	if err != nil {
		logger.Warning("parse xray versions failed, using fallback versions:", err)
		return fallbackXrayVersions, nil
	}
	versions := filterXrayVersions(releases)
	if len(versions) == 0 {
		logger.Warning("xray version list is empty, using fallback versions")
		return fallbackXrayVersions, nil
	}
	return versions, nil
}

func filterXrayVersions(releases []Release) []string {
	versions := make([]string, 0, len(releases))
	seen := make(map[string]bool, len(releases))
	for _, release := range releases {
		version := strings.TrimSpace(release.TagName)
		if release.Draft || release.Prerelease || !xrayVersionPattern.MatchString(version) || seen[version] {
			continue
		}
		seen[version] = true
		versions = append(versions, version)
	}
	return versions
}

func (s *ServerService) downloadXRay(version string) (string, error) {
	if !xrayVersionPattern.MatchString(version) || strings.ContainsAny(version, "/\\") {
		return "", errors.New("invalid xray version")
	}
	osName := runtime.GOOS
	arch := runtime.GOARCH

	switch osName {
	case "darwin":
		osName = "macos"
	}

	switch arch {
	case "amd64":
		arch = "64"
	case "arm64":
		arch = "arm64-v8a"
	}

	fileName := fmt.Sprintf("Xray-%s-%s.zip", osName, arch)
	url := fmt.Sprintf("https://github.com/XTLS/Xray-core/releases/download/%s/%s", version, fileName)
	resp, err := httpClient.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download xray returned status %d", resp.StatusCode)
	}
	if resp.ContentLength > maxXrayDownloadSize {
		return "", errors.New("download xray package is too large")
	}

	// 使用临时目录存放下载的 zip，避免 systemd ProtectSystem=full 导致 cwd 只读。
	tmpDir := os.TempDir()
	zipPath := filepath.Join(tmpDir, fileName)
	os.Remove(zipPath)
	file, err := os.OpenFile(zipPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return "", err
	}
	defer file.Close()

	written, err := io.Copy(file, io.LimitReader(resp.Body, maxXrayDownloadSize+1))
	if err != nil {
		return "", err
	}
	if written > maxXrayDownloadSize {
		return "", errors.New("download xray package is too large")
	}
	if err := verifyXrayDigest(version, zipPath); err != nil {
		// dgst 文件不存在时跳过校验（旧版本 release 可能没有 .dgst），保留已下载的 zip
		if strings.Contains(err.Error(), "status 404") {
			logger.Warning("xray digest not available for", version, ", skipping verification")
		} else {
			os.Remove(zipPath)
			return "", err
		}
	}

	return zipPath, nil
}

func verifyXrayDigest(version string, fileName string) error {
	assetName := filepath.Base(fileName)
	digestURL := fmt.Sprintf("https://github.com/XTLS/Xray-core/releases/download/%s/%s.dgst", version, assetName)
	resp, err := httpClient.Get(digestURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download xray digest returned status %d", resp.StatusCode)
	}

	digestBody, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return err
	}
	expected := extractSHA256Digest(string(digestBody), assetName)
	if expected == "" {
		return errors.New("xray digest does not contain sha256 checksum")
	}

	file, err := os.Open(fileName)
	if err != nil {
		return err
	}
	defer file.Close()
	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return err
	}
	actual := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(actual, expected) {
		return errors.New("xray package sha256 checksum mismatch")
	}
	return nil
}

func extractSHA256Digest(digestText string, fileName string) string {
	for _, line := range strings.Split(digestText, "\n") {
		lower := strings.ToLower(line)
		if strings.Contains(lower, "sha2-256") || strings.Contains(lower, "sha256") {
			if strings.Contains(line, fileName) || !strings.Contains(line, "Xray-") {
				if digest := sha256HexPattern.FindString(line); digest != "" {
					return digest
				}
			}
		}
	}
	return ""
}

type xrayPackageFile struct {
	zipName string
	path    string
	perm    fs.FileMode
}

func xrayPackageFiles() []xrayPackageFile {
	return []xrayPackageFile{
		{zipName: "xray", path: xray.GetBinaryPath(), perm: 0755},
		{zipName: "geosite.dat", path: xray.GetGeositePath(), perm: 0644},
		{zipName: "geoip.dat", path: xray.GetGeoipPath(), perm: 0644},
	}
}

func findZipFile(reader *zip.Reader, zipName string) *zip.File {
	for _, f := range reader.File {
		if f.Name == zipName {
			return f
		}
	}
	return nil
}

func extractZipFileToTemp(reader *zip.Reader, item xrayPackageFile) (string, error) {
	target := findZipFile(reader, item.zipName)
	if target == nil {
		return "", fmt.Errorf("xray package missing %s", item.zipName)
	}
	if target.FileInfo().IsDir() || target.UncompressedSize64 > maxXrayFileSize {
		return "", fmt.Errorf("xray package contains invalid %s", item.zipName)
	}

	zipFile, err := target.Open()
	if err != nil {
		return "", err
	}
	defer zipFile.Close()

	tmpFile, err := os.CreateTemp(filepath.Dir(item.path), "."+filepath.Base(item.path)+"-*.tmp")
	if err != nil {
		return "", err
	}
	tmpPath := tmpFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			os.Remove(tmpPath)
		}
	}()

	written, err := io.Copy(tmpFile, io.LimitReader(zipFile, maxXrayFileSize+1))
	if err != nil {
		tmpFile.Close()
		return "", err
	}
	if written > maxXrayFileSize {
		tmpFile.Close()
		return "", fmt.Errorf("xray package file %s is too large", item.zipName)
	}
	if err := tmpFile.Chmod(item.perm); err != nil {
		tmpFile.Close()
		return "", err
	}
	if err := tmpFile.Close(); err != nil {
		return "", err
	}
	cleanup = false
	return tmpPath, nil
}

func prepareXrayPackageFiles(reader *zip.Reader, files []xrayPackageFile) (map[string]string, error) {
	temps := make(map[string]string, len(files))
	cleanup := true
	defer func() {
		if cleanup {
			for _, tmpPath := range temps {
				os.Remove(tmpPath)
			}
		}
	}()

	for _, item := range files {
		tmpPath, err := extractZipFileToTemp(reader, item)
		if err != nil {
			return nil, err
		}
		temps[item.path] = tmpPath
	}
	cleanup = false
	return temps, nil
}

func replaceXrayPackageFiles(files []xrayPackageFile, temps map[string]string) (map[string]string, error) {
	backups := make(map[string]string, len(files))
	for _, item := range files {
		tmpPath := temps[item.path]
		backupPath := item.path + ".bak"
		os.Remove(backupPath)
		if _, err := os.Stat(item.path); err == nil {
			if err := os.Rename(item.path, backupPath); err != nil {
				return backups, err
			}
			backups[item.path] = backupPath
		} else if !os.IsNotExist(err) {
			return backups, err
		}
		if err := os.Rename(tmpPath, item.path); err != nil {
			return backups, err
		}
	}
	return backups, nil
}

func rollbackXrayPackageFiles(files []xrayPackageFile, backups map[string]string) error {
	var errs []string
	for i := len(files) - 1; i >= 0; i-- {
		path := files[i].path
		backupPath, ok := backups[path]
		if !ok {
			continue
		}
		os.Remove(path)
		if err := os.Rename(backupPath, path); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func cleanupXrayPackageBackups(backups map[string]string) {
	for _, backupPath := range backups {
		os.Remove(backupPath)
	}
}

func (s *ServerService) UpdateXray(version string) error {
	zipFileName, err := s.downloadXRay(version)
	if err != nil {
		return err
	}

	zipFile, err := os.Open(zipFileName)
	if err != nil {
		return err
	}
	defer func() {
		zipFile.Close()
		os.Remove(zipFileName)
	}()

	stat, err := zipFile.Stat()
	if err != nil {
		return err
	}
	reader, err := zip.NewReader(zipFile, stat.Size())
	if err != nil {
		return err
	}

	files := xrayPackageFiles()
	temps, err := prepareXrayPackageFiles(reader, files)
	if err != nil {
		return err
	}
	defer func() {
		for _, tmpPath := range temps {
			os.Remove(tmpPath)
		}
	}()

	_ = s.xrayService.StopXray()
	backups, err := replaceXrayPackageFiles(files, temps)
	if err != nil {
		_ = rollbackXrayPackageFiles(files, backups)
		_ = s.xrayService.RestartXray(true)
		return err
	}

	if err := s.xrayService.RestartXray(true); err != nil {
		rollbackErr := rollbackXrayPackageFiles(files, backups)
		restartErr := s.xrayService.RestartXray(true)
		if rollbackErr != nil {
			return fmt.Errorf("start xray failed: %v; rollback failed: %v", err, rollbackErr)
		}
		if restartErr != nil {
			return fmt.Errorf("start xray failed: %v; rollback restart failed: %v", err, restartErr)
		}
		return err
	}

	cleanupXrayPackageBackups(backups)
	return nil
}
