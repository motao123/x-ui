package controller

import (
	"x-ui/web/service"

	"github.com/gin-gonic/gin"
)

type QuickNodeController struct {
	quickNodeService service.QuickNodeService
}

func NewQuickNodeController(g *gin.RouterGroup) *QuickNodeController {
	a := &QuickNodeController{}
	a.initRouter(g)
	return a
}

func (a *QuickNodeController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/node")
	g.POST("/quick-vless-reality", a.quickVLESSReality)
}

func (a *QuickNodeController) quickVLESSReality(c *gin.Context) {
	request := &service.QuickRealityRequest{}
	if err := c.ShouldBind(request); err != nil {
		jsonMsg(c, "创建 Reality 节点", err)
		return
	}
	result, err := a.quickNodeService.CreateVLESSReality(request)
	securityLog(c, "quick_vless_reality", err == nil, " inbound_id=", resultInboundId(result))
	jsonObj(c, result, err)
}

func resultInboundId(result *service.QuickRealityResult) int {
	if result == nil {
		return 0
	}
	return result.InboundId
}
