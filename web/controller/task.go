package controller

import (
	"x-ui/web/service"

	"github.com/gin-gonic/gin"
)

type TaskController struct {
	taskService service.TaskService
}

func NewTaskController(g *gin.RouterGroup) *TaskController {
	a := &TaskController{}
	a.initRouter(g)
	return a
}

func (a *TaskController) initRouter(g *gin.RouterGroup) {
	g = g.Group("/task")
	g.POST("/list", a.list)
	g.POST("/get/:id", a.get)
}

func (a *TaskController) list(c *gin.Context) {
	jsonObj(c, a.taskService.List(), nil)
}

func (a *TaskController) get(c *gin.Context) {
	task, ok := a.taskService.Get(c.Param("id"))
	if !ok {
		jsonObj(c, nil, serviceError("任务不存在或已过期"))
		return
	}
	jsonObj(c, task, nil)
}

type serviceError string

func (e serviceError) Error() string { return string(e) }
