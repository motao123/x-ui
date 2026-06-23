package controller

import (
	"strconv"
	"x-ui/web/service"

	"github.com/gin-gonic/gin"
)

type ProxyUserController struct {
	BaseController
	proxyUserService service.ProxyUserService
}

func NewProxyUserController(g *gin.RouterGroup) *ProxyUserController {
	a := &ProxyUserController{}
	a.initRouter(g)
	return a
}

func (a *ProxyUserController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/proxy-user")
	g.POST("/list", a.list)
	g.POST("/save", a.save)
	g.POST("/del/:id", a.del)
	g.POST("/rotateToken/:id", a.rotateToken)
	g.POST("/resetTraffic/:id", a.resetTraffic)
}

func (a *ProxyUserController) list(c *gin.Context) {
	users, err := a.proxyUserService.List()
	jsonObj(c, users, err)
}

func (a *ProxyUserController) save(c *gin.Context) {
	payload := &service.ProxyUserPayload{}
	if err := c.ShouldBind(payload); err != nil {
		jsonMsg(c, "保存代理用户", err)
		return
	}
	err := a.proxyUserService.Save(payload)
	securityLog(c, "proxy_user_save", err == nil, " proxy_user_id=", payload.Id)
	jsonMsg(c, "保存代理用户", err)
}

func (a *ProxyUserController) del(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "删除代理用户", err)
		return
	}
	err = a.proxyUserService.Delete(id)
	securityLog(c, "proxy_user_delete", err == nil, " proxy_user_id=", id)
	jsonMsg(c, "删除代理用户", err)
}

func (a *ProxyUserController) rotateToken(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "重置订阅令牌", err)
		return
	}
	token, err := a.proxyUserService.RotateToken(id)
	securityLog(c, "proxy_user_rotate_token", err == nil, " proxy_user_id=", id)
	jsonObj(c, gin.H{"token": token}, err)
}

func (a *ProxyUserController) resetTraffic(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "重置代理用户流量", err)
		return
	}
	err = a.proxyUserService.ResetTraffic(id)
	securityLog(c, "proxy_user_reset_traffic", err == nil, " proxy_user_id=", id)
	jsonMsg(c, "重置代理用户流量", err)
}
