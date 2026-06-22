package controller

import (
	"x-ui/web/service"

	"github.com/gin-gonic/gin"
)

type DetectController struct {
	BaseController
	detectService service.DetectService
}

type detectRequest struct {
	Force     bool     `form:"force" json:"force"`
	Platforms []string `form:"platforms" json:"platforms"`
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
