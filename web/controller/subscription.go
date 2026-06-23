package controller

import (
	"net"
	"strings"
	"x-ui/web/service"

	"github.com/gin-gonic/gin"
)

type SubscriptionController struct {
	subscriptionService service.SubscriptionService
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
	body, contentType, err := a.subscriptionService.Render(token, format, publicHost(c), c.GetHeader("User-Agent"), getRemoteIp(c))
	if err != nil {
		c.String(404, "subscription not found")
		return
	}
	c.Header("Content-Type", contentType)
	c.String(200, body)
}

func publicHost(c *gin.Context) string {
	host := c.Request.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return host
}
