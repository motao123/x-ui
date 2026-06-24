package controller

import (
	"strings"
	"x-ui/web/service"

	"github.com/gin-gonic/gin"
)

type SubscriptionController struct {
	subscriptionService service.SubscriptionService
	settingService      service.SettingService
}

func NewSubscriptionController(g *gin.RouterGroup) *SubscriptionController {
	a := &SubscriptionController{}
	a.initRouter(g)
	return a
}

func (a *SubscriptionController) initRouter(g *gin.RouterGroup) {
	g.GET("/sub/:token", a.render)
	g.GET("/sub/:token/:format", a.render)
}

func (a *SubscriptionController) render(c *gin.Context) {
	token := strings.TrimSpace(c.Param("token"))
	format := strings.TrimSpace(c.Param("format"))
	host, err := a.settingService.GetSubscriptionHost()
	if err != nil {
		c.String(404, "subscription not found")
		return
	}
	body, contentType, err := a.subscriptionService.Render(token, format, host, c.GetHeader("User-Agent"), getRemoteIp(c))
	if err != nil {
		c.String(404, "subscription not found")
		return
	}
	c.Header("Content-Type", contentType)
	c.String(200, body)
}
