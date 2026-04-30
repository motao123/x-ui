package service

import (
	"encoding/json"
	"errors"
	"sync"
	"x-ui/logger"
	"x-ui/xray"

	"go.uber.org/atomic"
)

var p *xray.Process
var lock sync.Mutex
var isNeedXrayRestart atomic.Bool
var result string

type XrayService struct {
	inboundService InboundService
	settingService SettingService
}

func (s *XrayService) IsXrayRunning() bool {
	lock.Lock()
	defer lock.Unlock()
	return p != nil && p.IsRunning()
}

func (s *XrayService) GetXrayErr() error {
	lock.Lock()
	defer lock.Unlock()
	if p == nil {
		return nil
	}
	return p.GetErr()
}

func (s *XrayService) GetXrayResult() string {
	lock.Lock()
	defer lock.Unlock()
	if result != "" {
		return result
	}
	if p != nil && p.IsRunning() {
		return ""
	}
	if p == nil {
		return ""
	}
	result = p.GetResult()
	return result
}

func (s *XrayService) GetXrayVersion() string {
	lock.Lock()
	defer lock.Unlock()
	if p == nil {
		return "Unknown"
	}
	return p.GetVersion()
}

func (s *XrayService) GetXrayConfig() (*xray.Config, error) {
	templateConfig, err := s.settingService.GetXrayConfigTemplate()
	if err != nil {
		return nil, err
	}

	xrayConfig := &xray.Config{}
	err = json.Unmarshal([]byte(templateConfig), xrayConfig)
	if err != nil {
		return nil, err
	}

	inbounds, err := s.inboundService.GetAllInbounds()
	if err != nil {
		return nil, err
	}
	for _, inbound := range inbounds {
		if !inbound.Enable {
			continue
		}
		inboundConfig := inbound.GenXrayInboundConfig()
		xrayConfig.InboundConfigs = append(xrayConfig.InboundConfigs, *inboundConfig)
	}
	return xrayConfig, nil
}

func (s *XrayService) GetXrayTraffic() ([]*xray.Traffic, error) {
	lock.Lock()
	if p == nil || !p.IsRunning() {
		lock.Unlock()
		return nil, errors.New("xray is not running")
	}
	currentP := p
	lock.Unlock()
	return currentP.GetTraffic(true)
}

func (s *XrayService) RestartXray(isForce bool) error {
	lock.Lock()
	defer lock.Unlock()
	logger.Debug("restart xray, force:", isForce)

	xrayConfig, err := s.GetXrayConfig()
	if err != nil {
		return err
	}

	if p != nil && p.IsRunning() {
		if !isForce && p.GetConfig().Equals(xrayConfig) {
			logger.Debug("not need to restart xray")
			return nil
		}
	}

	// 先验证配置，避免错误配置导致服务中断
	err = xray.TestConfig(xrayConfig)
	if err != nil {
		logger.Debug("xray config test failed:", err)
		return err
	}

	// 保存旧进程引用用于回滚
	oldProcess := p

	// 停止旧进程
	if oldProcess != nil && oldProcess.IsRunning() {
		oldProcess.Stop()
	}

	// 启动新进程
	newProcess := xray.NewProcess(xrayConfig)
	result = ""
	err = newProcess.Start()
	if err != nil {
		// 启动失败，尝试恢复旧配置
		logger.Debug("xray start failed, trying rollback:", err)
		if oldProcess != nil && oldProcess.GetConfig() != nil {
			rollbackProcess := xray.NewProcess(oldProcess.GetConfig())
			rollbackErr := rollbackProcess.Start()
			if rollbackErr != nil {
				logger.Debug("xray rollback also failed:", rollbackErr)
			} else {
				p = rollbackProcess
				logger.Debug("xray rollback succeeded")
			}
		}
		return err
	}

	p = newProcess
	return nil
}

func (s *XrayService) StopXray() error {
	lock.Lock()
	defer lock.Unlock()
	logger.Debug("stop xray")
	if p != nil && p.IsRunning() {
		return p.Stop()
	}
	return errors.New("xray is not running")
}

func (s *XrayService) SetToNeedRestart() {
	isNeedXrayRestart.Store(true)
}

func (s *XrayService) IsNeedRestartAndSetFalse() bool {
	return isNeedXrayRestart.CAS(true, false)
}
