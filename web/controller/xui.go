package controller

import (
	"github.com/gin-gonic/gin"
)

type XUIController struct {
	BaseController

	inboundController *InboundController
	settingController *SettingController
	detectController  *DetectController
}

func NewXUIController(g *gin.RouterGroup) *XUIController {
	a := &XUIController{}
	a.initRouter(g)
	return a
}

func (a *XUIController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/xui")
	g.Use(a.checkLogin)

	g.GET("/", a.index)
	g.GET("/inbounds", a.inbounds)
	g.GET("/detect", a.detect)
	g.GET("/logs", a.logs)
	g.GET("/setting", a.setting)

	a.inboundController = NewInboundController(g)
	a.settingController = NewSettingController(g)
	a.detectController = NewDetectController(g)
}

func (a *XUIController) index(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	html(c, "index.html", "系统状态", nil)
}

func (a *XUIController) inbounds(c *gin.Context) {
	html(c, "inbounds.html", "入站列表", nil)
}

func (a *XUIController) detect(c *gin.Context) {
	html(c, "detect.html", "网络检测", nil)
}

func (a *XUIController) logs(c *gin.Context) {
	html(c, "logs.html", "面板日志", nil)
}

func (a *XUIController) setting(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	html(c, "setting.html", "设置", nil)
}
