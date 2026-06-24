package controller

import (
	"strconv"
	"x-ui/database/model"
	"x-ui/web/service"

	"github.com/gin-gonic/gin"
)

type EndpointController struct {
	endpointService service.EndpointService
	warpService     service.WarpService
	taskService     service.TaskService
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
	g.POST("/warp", a.warp)
	g.POST("/warp/register", a.registerWarp)
	g.POST("/warp/refresh", a.refreshWarp)
	g.POST("/warp/license", a.setWarpLicense)
	g.POST("/warp/autoUpdate", a.setWarpAutoUpdate)
	g.POST("/warp/delete", a.deleteWarp)
	g.POST("/warp/createEndpoint", a.createWarpEndpoint)
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

func (a *EndpointController) warp(c *gin.Context) {
	account, err := a.warpService.Get()
	jsonObj(c, account, err)
}

func (a *EndpointController) registerWarp(c *gin.Context) {
	task := a.taskService.Start("注册 WARP 账号", func(task *service.Task) {
		data, err := a.warpService.Register()
		if err != nil {
			task.Fail(err.Error())
			return
		}
		task.Log("INFO", "WARP 账号注册成功，Peer: "+data.PeerEndpoint)
		task.Done("任务完成")
	})
	securityLog(c, "warp_register", true)
	jsonObj(c, task, nil)
}

func (a *EndpointController) refreshWarp(c *gin.Context) {
	data, err := a.warpService.Refresh()
	securityLog(c, "warp_refresh", err == nil)
	jsonObj(c, data, err)
}

func (a *EndpointController) setWarpLicense(c *gin.Context) {
	var req struct {
		License string `json:"license" form:"license"`
	}
	if err := c.ShouldBind(&req); err != nil {
		jsonMsg(c, "设置 WARP License", err)
		return
	}
	data, err := a.warpService.SetLicense(req.License)
	securityLog(c, "warp_license", err == nil)
	jsonObj(c, data, err)
}

func (a *EndpointController) setWarpAutoUpdate(c *gin.Context) {
	var req struct {
		Days int `json:"days" form:"days"`
	}
	if err := c.ShouldBind(&req); err != nil {
		jsonMsg(c, "设置 WARP 自动更新", err)
		return
	}
	account, err := a.warpService.SetAutoUpdate(req.Days)
	securityLog(c, "warp_auto_update", err == nil)
	jsonObj(c, account, err)
}

func (a *EndpointController) deleteWarp(c *gin.Context) {
	err := a.warpService.Delete()
	securityLog(c, "warp_delete", err == nil)
	jsonMsg(c, "删除 WARP 账号", err)
}

func (a *EndpointController) createWarpEndpoint(c *gin.Context) {
	var req struct {
		Tag string `json:"tag" form:"tag"`
	}
	_ = c.ShouldBind(&req)
	endpoint, _, err := a.warpService.CreateEndpoint(req.Tag)
	securityLog(c, "warp_create_endpoint", err == nil, " tag=", req.Tag)
	jsonObj(c, endpoint, err)
}
