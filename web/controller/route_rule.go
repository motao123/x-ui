package controller

import (
	"strconv"
	"x-ui/database/model"
	"x-ui/web/service"

	"github.com/gin-gonic/gin"
)

type RouteRuleController struct {
	routeRuleService service.RouteRuleService
	xrayService      service.XrayService
}

func NewRouteRuleController(g *gin.RouterGroup) *RouteRuleController {
	a := &RouteRuleController{}
	a.initRouter(g)
	return a
}

func (a *RouteRuleController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/route-rule")
	g.POST("/list", a.list)
	g.POST("/save", a.save)
	g.POST("/del/:id", a.del)
	g.POST("/apply", a.apply)
}

func (a *RouteRuleController) list(c *gin.Context) {
	rules, err := a.routeRuleService.List()
	jsonObj(c, rules, err)
}

func (a *RouteRuleController) save(c *gin.Context) {
	rule := &model.RouteRule{}
	if err := c.ShouldBind(rule); err != nil {
		jsonMsg(c, "保存路由规则", err)
		return
	}
	err := a.routeRuleService.Save(rule)
	securityLog(c, "route_rule_save", err == nil, " route_rule_id=", rule.Id)
	jsonMsg(c, "保存路由规则", err)
}

func (a *RouteRuleController) del(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		jsonMsg(c, "删除路由规则", err)
		return
	}
	err = a.routeRuleService.Delete(id)
	securityLog(c, "route_rule_delete", err == nil, " route_rule_id=", id)
	jsonMsg(c, "删除路由规则", err)
}

func (a *RouteRuleController) apply(c *gin.Context) {
	err := a.xrayService.RestartXray(true)
	securityLog(c, "route_rule_apply", err == nil)
	jsonMsg(c, "应用路由规则", err)
}
