package controller

import (
	"encoding/json"
	"x-ui/web/service"
	"x-ui/xray"

	"github.com/gin-gonic/gin"
)

type DetectController struct {
	BaseController
	detectService service.DetectService
	xrayService   service.XrayService
}

type detectRequest struct {
	Force     bool     `form:"force" json:"force"`
	Platforms []string `form:"platforms" json:"platforms"`
}

type protocolDetectResult struct {
	Valid    bool                    `json:"valid"`
	Error    string                  `json:"error"`
	Inbounds []protocolDetectInbound `json:"inbounds"`
}

type protocolDetectInbound struct {
	Tag      string `json:"tag"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
	Network  string `json:"network"`
	Security string `json:"security"`
}

func NewDetectController(g *gin.RouterGroup) *DetectController {
	a := &DetectController{}
	a.initRouter(g)
	return a
}

func (a *DetectController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/detect")
	g.POST("/ip", a.ip)
	g.POST("/unlock", a.unlock)
	g.POST("/all", a.all)
	g.POST("/protocols", a.protocols)
	g.POST("/backroute", a.backRoute)
}

func (a *DetectController) ip(c *gin.Context) {
	var request detectRequest
	_ = c.ShouldBind(&request)

	result := a.detectService.DetectIP(request.Force)
	securityLog(c, "detect_ip", result.Error == "", " force=", request.Force)
	jsonObj(c, result, nil)
}

func (a *DetectController) unlock(c *gin.Context) {
	var request detectRequest
	_ = c.ShouldBind(&request)

	result := a.detectService.DetectUnlock(request.Platforms, request.Force)
	securityLog(c, "detect_unlock", true, " force=", request.Force)
	jsonObj(c, result, nil)
}

func (a *DetectController) all(c *gin.Context) {
	var request detectRequest
	_ = c.ShouldBind(&request)

	result := a.detectService.DetectAll(request.Platforms, request.Force)
	securityLog(c, "detect_all", result.IP == nil || result.IP.Error == "", " force=", request.Force)
	jsonObj(c, result, nil)
}

func (a *DetectController) protocols(c *gin.Context) {
	config, err := a.xrayService.GetXrayConfig()
	if err != nil {
		securityLog(c, "detect_protocols", false)
		jsonObj(c, &protocolDetectResult{Valid: false, Error: "读取 Xray 配置失败"}, nil)
		return
	}

	result := &protocolDetectResult{
		Valid:    true,
		Inbounds: make([]protocolDetectInbound, 0, len(config.InboundConfigs)),
	}
	for _, inbound := range config.InboundConfigs {
		network, security := detectStreamInfo(inbound.StreamSettings)
		result.Inbounds = append(result.Inbounds, protocolDetectInbound{
			Tag:      inbound.Tag,
			Port:     inbound.Port,
			Protocol: inbound.Protocol,
			Network:  network,
			Security: security,
		})
	}

	if err := xray.TestConfig(config); err != nil {
		result.Valid = false
		result.Error = "Xray 配置验证失败，请查看面板日志"
	}
	securityLog(c, "detect_protocols", result.Valid)
	jsonObj(c, result, nil)
}

func (a *DetectController) backRoute(c *gin.Context) {
	result := a.detectService.DetectBackRoute()
	securityLog(c, "detect_backroute", true)
	jsonObj(c, result, nil)
}

func detectStreamInfo(raw []byte) (string, string) {
	var stream struct {
		Network  string `json:"network"`
		Security string `json:"security"`
	}
	if len(raw) == 0 || json.Unmarshal(raw, &stream) != nil {
		return "", ""
	}
	return stream.Network, stream.Security
}
