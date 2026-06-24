package controller

import (
	"encoding/json"
	"fmt"
	"strings"
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

type detectOverviewResult struct {
	Score       int                          `json:"score"`
	Level       string                       `json:"level"`
	Metrics     []detectOverviewMetric       `json:"metrics"`
	IP          *service.DetectIPResult      `json:"ip"`
	Unlock      []service.DetectUnlockResult `json:"unlock"`
	Protocols   *protocolDetectResult        `json:"protocols"`
	BackRoute   []service.BackRouteResult    `json:"backRoute"`
	Suggestions []string                     `json:"suggestions"`
	CheckedAt   int64                        `json:"checkedAt"`
}

type detectOverviewMetric struct {
	Key    string `json:"key"`
	Title  string `json:"title"`
	Value  string `json:"value"`
	Status string `json:"status"`
	Hint   string `json:"hint"`
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
	g.POST("/overview", a.overview)
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

func (a *DetectController) overview(c *gin.Context) {
	var request detectRequest
	_ = c.ShouldBind(&request)

	ip := a.detectService.DetectIP(request.Force)
	unlock := a.detectService.DetectUnlock(request.Platforms, request.Force)
	protocols := a.buildProtocolResult()
	backRoute := a.detectService.DetectBackRoute()
	result := buildDetectOverview(ip, unlock, protocols, backRoute)
	securityLog(c, "detect_overview", ip == nil || ip.Error == "", " force=", request.Force)
	jsonObj(c, result, nil)
}

func (a *DetectController) protocols(c *gin.Context) {
	result := a.buildProtocolResult()
	securityLog(c, "detect_protocols", result.Valid)
	jsonObj(c, result, nil)
}

func (a *DetectController) buildProtocolResult() *protocolDetectResult {
	config, err := a.xrayService.GetXrayConfig()
	if err != nil {
		return &protocolDetectResult{Valid: false, Error: "读取 Xray 配置失败"}
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
	return result
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

func buildDetectOverview(ip *service.DetectIPResult, unlock []service.DetectUnlockResult, protocols *protocolDetectResult, backRoute []service.BackRouteResult) *detectOverviewResult {
	score := 100
	suggestions := []string{}
	metrics := []detectOverviewMetric{}

	if ip == nil || ip.Error != "" {
		score -= 25
		suggestions = append(suggestions, "IP 信息检测失败，建议检查服务器出站网络和 DNS。")
		metrics = append(metrics, detectOverviewMetric{Key: "ip", Title: "出口 IP", Value: "检测失败", Status: "error", Hint: errorText(ip)})
	} else {
		ipStatus := "success"
		if strings.Contains(strings.ToLower(ip.IPType), "hosting") || strings.Contains(strings.ToLower(ip.IPType), "proxy") {
			ipStatus = "warning"
			score -= 5
			suggestions = append(suggestions, "当前出口 IP 类型偏机房/代理，部分平台可能风控更严格。")
		}
		metrics = append(metrics, detectOverviewMetric{Key: "ip", Title: "出口 IP", Value: firstDetectText(ip.IP, "-"), Status: ipStatus, Hint: strings.TrimSpace(ip.Country + " " + ip.City + " " + ip.Org)})
	}

	unlocked, blocked, unknown := countUnlockStatus(unlock)
	if len(unlock) == 0 {
		score -= 20
		suggestions = append(suggestions, "平台解锁未检测到结果，建议重新强制检测。")
	} else {
		score -= blocked * 6
		score -= unknown * 2
		if blocked > 0 {
			suggestions = append(suggestions, fmt.Sprintf("有 %d 个平台受限，可考虑切换出口或使用 WARP/路由分流。", blocked))
		}
	}
	metrics = append(metrics, detectOverviewMetric{Key: "unlock", Title: "平台可达", Value: fmt.Sprintf("%d/%d", unlocked, len(unlock)), Status: statusByCounts(blocked, unknown), Hint: fmt.Sprintf("受限 %d，未知 %d", blocked, unknown)})

	protocolStatus := "success"
	protocolValue := "通过"
	if protocols == nil || !protocols.Valid {
		protocolStatus = "error"
		protocolValue = "异常"
		score -= 25
		suggestions = append(suggestions, "Xray 配置校验失败，请先查看入站配置和面板日志。")
	}
	protocolCount := 0
	if protocols != nil {
		protocolCount = len(protocols.Inbounds)
	}
	metrics = append(metrics, detectOverviewMetric{Key: "protocols", Title: "协议配置", Value: protocolValue, Status: protocolStatus, Hint: fmt.Sprintf("%d 个启用入站", protocolCount)})

	backRouteStatus := "success"
	backRouteValue := "已检测"
	if hasBackRouteError(backRoute) {
		backRouteStatus = "warning"
		backRouteValue = "需安装 mtr"
		score -= 8
		suggestions = append(suggestions, "回程检测需要 mtr，Debian/Ubuntu 可安装 mtr-tiny。")
	} else if len(backRoute) == 0 {
		backRouteStatus = "default"
		backRouteValue = "未检测"
	}
	metrics = append(metrics, detectOverviewMetric{Key: "backRoute", Title: "三网回程", Value: backRouteValue, Status: backRouteStatus, Hint: backRouteHint(backRoute)})

	if score < 0 {
		score = 0
	}
	if len(suggestions) == 0 {
		suggestions = append(suggestions, "网络状态整体正常，可按需强制刷新获取最新结果。")
	}

	return &detectOverviewResult{Score: score, Level: detectScoreLevel(score), Metrics: metrics, IP: ip, Unlock: unlock, Protocols: protocols, BackRoute: backRoute, Suggestions: suggestions, CheckedAt: nowUnixSafe(ip, unlock, backRoute)}
}

func countUnlockStatus(results []service.DetectUnlockResult) (int, int, int) {
	unlocked, blocked, unknown := 0, 0, 0
	for _, result := range results {
		switch result.Status {
		case "unlocked":
			unlocked++
		case "blocked", "timeout":
			blocked++
		default:
			unknown++
		}
	}
	return unlocked, blocked, unknown
}

func statusByCounts(blocked int, unknown int) string {
	if blocked > 0 {
		return "error"
	}
	if unknown > 0 {
		return "warning"
	}
	return "success"
}

func detectScoreLevel(score int) string {
	if score >= 90 {
		return "优秀"
	}
	if score >= 75 {
		return "良好"
	}
	if score >= 60 {
		return "一般"
	}
	return "需处理"
}

func hasBackRouteError(rows []service.BackRouteResult) bool {
	for _, row := range rows {
		if row.Error != "" {
			return true
		}
	}
	return false
}

func backRouteHint(rows []service.BackRouteResult) string {
	if len(rows) == 0 {
		return "暂无结果"
	}
	if hasBackRouteError(rows) {
		return rows[0].Error
	}
	return fmt.Sprintf("%d 个城市", len(rows))
}

func errorText(ip *service.DetectIPResult) string {
	if ip == nil || ip.Error == "" {
		return ""
	}
	return ip.Error
}

func firstDetectText(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func nowUnixSafe(ip *service.DetectIPResult, unlock []service.DetectUnlockResult, backRoute []service.BackRouteResult) int64 {
	if ip != nil && ip.CheckedAt > 0 {
		return ip.CheckedAt
	}
	if len(unlock) > 0 {
		return unlock[0].CheckedAt
	}
	if len(backRoute) > 0 {
		return backRoute[0].UpdatedAt
	}
	return 0
}
