package controller

import (
	"strconv"
	"x-ui/database/model"
	"x-ui/web/service"

	"github.com/gin-gonic/gin"
)

type EndpointController struct {
	endpointService service.EndpointService
	xrayService     service.XrayService
}

func NewEndpointController(g *gin.RouterGroup) *EndpointController {
	a := &EndpointController{}
	a.initRouter(g)
	return a
}

func (a *EndpointController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/endpoint")
	g.POST("/list", a.list)
	g.POST("/save", a.save)
	g.POST("/del/:id", a.del)
	g.POST("/apply", a.apply)
}

func (a *EndpointController) list(c *gin.Context) {
	endpoints, err := a.endpointService.List()
	jsonObj(c, endpoints, err)
}

func (a *EndpointController) save(c *gin.Context) {
	endpoint := &model.Endpoint{}
	if err := c.ShouldBind(endpoint); err != nil {
		jsonMsg(c, "保存端点", err)
		return
	}
	err := a.endpointService.Save(endpoint)
	securityLog(c, "endpoint_save", err == nil, " endpoint_id=", endpoint.Id)
	jsonMsg(c, "保存端点", err)
}

func (a *EndpointController) del(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "删除端点", err)
		return
	}
	err = a.endpointService.Delete(id)
	securityLog(c, "endpoint_delete", err == nil, " endpoint_id=", id)
	jsonMsg(c, "删除端点", err)
}

func (a *EndpointController) apply(c *gin.Context) {
	err := a.xrayService.RestartXray(true)
	securityLog(c, "endpoint_apply", err == nil)
	jsonMsg(c, "应用端点", err)
}
