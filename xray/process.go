package xray

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
	"x-ui/util/common"

	"github.com/Workiva/go-datastructures/queue"
	statsservice "github.com/xtls/xray-core/app/stats/command"
	"google.golang.org/grpc"
)

var trafficRegex = regexp.MustCompile("(inbound|outbound)>>>([^>]+)>>>traffic>>>(downlink|uplink)")

func GetBinaryName() string {
	return fmt.Sprintf("xray-%s-%s", runtime.GOOS, runtime.GOARCH)
}

func GetBinaryPath() string {
	return "bin/" + GetBinaryName()
}

func GetConfigPath() string {
	return "bin/config.json"
}

func GetGeositePath() string {
	return "bin/geosite.dat"
}

func GetGeoipPath() string {
	return "bin/geoip.dat"
}

func stopProcess(p *Process) {
	p.Stop()
}

type Process struct {
	*process
}

func NewProcess(xrayConfig *Config) *Process {
	p := &Process{newProcess(xrayConfig)}
	runtime.SetFinalizer(p, stopProcess)
	return p
}

type process struct {
	cmd *exec.Cmd

	version string
	apiPort int

	config  *Config
	lines   *queue.Queue
	exitErr error
}

func newProcess(config *Config) *process {
	return &process{
		version: "Unknown",
		config:  config,
		lines:   queue.New(100),
	}
}

func (p *process) IsRunning() bool {
	if p.cmd == nil || p.cmd.Process == nil {
		return false
	}
	if p.cmd.ProcessState == nil {
		return true
	}
	return false
}

func (p *process) GetErr() error {
	return p.exitErr
}

func (p *process) GetResult() string {
	if p.lines.Empty() && p.exitErr != nil {
		return p.exitErr.Error()
	}
	items, _ := p.lines.TakeUntil(func(item interface{}) bool {
		return true
	})
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, item.(string))
	}
	return strings.Join(lines, "\n")
}

func (p *process) GetVersion() string {
	return p.version
}

func (p *Process) GetAPIPort() int {
	return p.apiPort
}

func (p *Process) GetConfig() *Config {
	return p.config
}

func (p *process) refreshAPIPort() {
	for _, inbound := range p.config.InboundConfigs {
		if inbound.Tag == "api" {
			p.apiPort = inbound.Port
			break
		}
	}
}

func (p *process) refreshVersion() {
	cmd := exec.Command(GetBinaryPath(), "-version")
	data, err := cmd.Output()
	if err != nil {
		p.version = "Unknown"
	} else {
		datas := bytes.Split(data, []byte(" "))
		if len(datas) <= 1 {
			p.version = "Unknown"
		} else {
			p.version = string(datas[1])
		}
	}
}

func (p *process) Start() (err error) {
	if p.IsRunning() {
		return errors.New("xray is already running")
	}

	defer func() {
		if err != nil {
			p.exitErr = err
		}
	}()

	data, err := json.MarshalIndent(p.config, "", "  ")
	if err != nil {
		return common.NewErrorf("生成 xray 配置文件失败: %v", err)
	}
	configPath := GetConfigPath()
	if err := atomicWriteFile(configPath, data, 0640); err != nil {
		return common.NewErrorf("写入配置文件失败: %v", err)
	}
	setConfigFileOwner(configPath)

	cmd := exec.Command(GetBinaryPath(), "-c", configPath)
	p.cmd = cmd
	setProcessUser(cmd)

	stdReader, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	errReader, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	go func() {
		defer func() {
			common.Recover("")
			stdReader.Close()
		}()
		reader := bufio.NewReaderSize(stdReader, 8192)
		for {
			line, _, err := reader.ReadLine()
			if err != nil {
				return
			}
			if p.lines.Len() >= 100 {
				p.lines.Get(1)
			}
			p.lines.Put(string(line))
		}
	}()

	go func() {
		defer func() {
			common.Recover("")
			errReader.Close()
		}()
		reader := bufio.NewReaderSize(errReader, 8192)
		for {
			line, _, err := reader.ReadLine()
			if err != nil {
				return
			}
			if p.lines.Len() >= 100 {
				p.lines.Get(1)
			}
			p.lines.Put(string(line))
		}
	}()

	go func() {
		err := cmd.Run()
		if err != nil {
			p.exitErr = err
		}
	}()

	p.refreshVersion()
	p.refreshAPIPort()

	return nil
}

func (p *process) Stop() error {
	if !p.IsRunning() {
		return errors.New("xray is not running")
	}
	// 优雅关闭：先 SIGTERM，2秒后 SIGKILL
	err := p.cmd.Process.Signal(os.Interrupt)
	if err != nil {
		return p.cmd.Process.Kill()
	}
	done := make(chan bool)
	go func() {
		p.cmd.Wait()
		done <- true
	}()
	select {
	case <-done:
		return nil
	case <-time.After(2 * time.Second):
		return p.cmd.Process.Kill()
	}
}

// atomicWriteFile 先写入临时文件再重命名，避免写入中断导致配置不完整
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, "*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return err
	}
	if err := tmpFile.Chmod(perm); err != nil {
		tmpFile.Close()
		os.Remove(tmpPath)
		return err
	}
	tmpFile.Close()
	return os.Rename(tmpPath, path)
}

// TestConfig 验证 xray 配置是否合法，不启动 xray
func TestConfig(config *Config) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return common.NewErrorf("生成 xray 配置失败: %v", err)
	}
	tmpFile, err := os.CreateTemp("", "xray-config-*.json")
	if err != nil {
		return common.NewErrorf("创建测试配置文件失败: %v", err)
	}
	testPath := tmpFile.Name()
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		os.Remove(testPath)
		return common.NewErrorf("写入测试配置文件失败: %v", err)
	}
	tmpFile.Close()
	defer os.Remove(testPath)

	cmd := exec.Command(GetBinaryPath(), "run", "-c", testPath, "-test")
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		if strings.Contains(outputStr, "Configuration OK") {
			return nil
		}
		return common.NewErrorf("xray 配置验证失败: %s", strings.TrimSpace(outputStr))
	}
	return nil
}

func (p *process) GetTraffic(reset bool) ([]*Traffic, error) {
	if p.apiPort == 0 {
		return nil, common.NewError("xray api port wrong:", p.apiPort)
	}
	conn, err := grpc.Dial(fmt.Sprintf("127.0.0.1:%v", p.apiPort), grpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := statsservice.NewStatsServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	request := &statsservice.QueryStatsRequest{
		Reset_: reset,
	}
	resp, err := client.QueryStats(ctx, request)
	if err != nil {
		return nil, err
	}
	tagTrafficMap := map[string]*Traffic{}
	traffics := make([]*Traffic, 0)
	for _, stat := range resp.GetStat() {
		matchs := trafficRegex.FindStringSubmatch(stat.Name)
		if matchs == nil {
			continue
		}
		isInbound := matchs[1] == "inbound"
		tag := matchs[2]
		isDown := matchs[3] == "downlink"
		if tag == "api" {
			continue
		}
		traffic, ok := tagTrafficMap[tag]
		if !ok {
			traffic = &Traffic{
				IsInbound: isInbound,
				Tag:       tag,
			}
			tagTrafficMap[tag] = traffic
			traffics = append(traffics, traffic)
		}
		if isDown {
			traffic.Down = stat.Value
		} else {
			traffic.Up = stat.Value
		}
	}

	return traffics, nil
}
